package collectors

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

const (
	schemaFingerprintVersion         = "v1"
	maxSchemaFingerprintFieldBytes   = 1 << 20
	maxSchemaFingerprintRecordBytes  = 4 << 20
	maxSchemaFingerprintRowsPerGroup = 100000
	maxSchemaFingerprintTotalBytes   = 64 << 20
)

const schemaFingerprintTablesSQL = `SELECT
  TABLE_SCHEMA,
  TABLE_NAME,
  TABLE_TYPE,
  COALESCE(ENGINE, ''),
  COALESCE(ROW_FORMAT, ''),
  COALESCE(TABLE_COLLATION, '')
FROM information_schema.tables
WHERE TABLE_SCHEMA = ?
ORDER BY TABLE_SCHEMA, TABLE_NAME`

const schemaFingerprintColumnsSQL = `SELECT
  TABLE_SCHEMA,
  TABLE_NAME,
  CAST(ORDINAL_POSITION AS CHAR),
  COLUMN_NAME,
  COLUMN_TYPE,
  IS_NULLABLE,
  IF(COLUMN_DEFAULT IS NULL, '1', '0'),
  COALESCE(CAST(COLUMN_DEFAULT AS CHAR), ''),
  EXTRA,
  COALESCE(COLLATION_NAME, ''),
  COALESCE(GENERATION_EXPRESSION, '')
FROM information_schema.columns
WHERE TABLE_SCHEMA = ?
ORDER BY TABLE_SCHEMA, TABLE_NAME, ORDINAL_POSITION`

const schemaFingerprintIndexesSQL = `SELECT
  TABLE_SCHEMA,
  TABLE_NAME,
  INDEX_NAME,
  CAST(NON_UNIQUE AS CHAR),
  CAST(SEQ_IN_INDEX AS CHAR),
  COALESCE(COLUMN_NAME, ''),
  COALESCE(EXPRESSION, ''),
  COALESCE(CAST(SUB_PART AS CHAR), ''),
  COALESCE(COLLATION, ''),
  INDEX_TYPE,
  IS_VISIBLE
FROM information_schema.statistics
WHERE TABLE_SCHEMA = ?
ORDER BY TABLE_SCHEMA, TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX`

const schemaFingerprintConstraintsSQL = `SELECT
  tc.CONSTRAINT_SCHEMA,
  tc.TABLE_NAME,
  tc.CONSTRAINT_NAME,
  tc.CONSTRAINT_TYPE,
  COALESCE(CAST(kcu.ORDINAL_POSITION AS CHAR), ''),
  COALESCE(kcu.COLUMN_NAME, ''),
  COALESCE(kcu.REFERENCED_TABLE_SCHEMA, ''),
  COALESCE(kcu.REFERENCED_TABLE_NAME, ''),
  COALESCE(kcu.REFERENCED_COLUMN_NAME, '')
FROM information_schema.table_constraints tc
LEFT JOIN information_schema.key_column_usage kcu
  ON kcu.CONSTRAINT_SCHEMA = tc.CONSTRAINT_SCHEMA
 AND kcu.TABLE_NAME = tc.TABLE_NAME
 AND kcu.CONSTRAINT_NAME = tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_SCHEMA = ?
ORDER BY tc.CONSTRAINT_SCHEMA, tc.TABLE_NAME, tc.CONSTRAINT_NAME, kcu.ORDINAL_POSITION`

const schemaFingerprintChecksSQL = `SELECT
  tc.CONSTRAINT_SCHEMA,
  tc.TABLE_NAME,
  tc.CONSTRAINT_NAME,
  cc.CHECK_CLAUSE
FROM information_schema.table_constraints tc
JOIN information_schema.check_constraints cc
  ON cc.CONSTRAINT_SCHEMA = tc.CONSTRAINT_SCHEMA
 AND cc.CONSTRAINT_NAME = tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_SCHEMA = ?
  AND tc.CONSTRAINT_TYPE = 'CHECK'
ORDER BY tc.CONSTRAINT_SCHEMA, tc.TABLE_NAME, tc.CONSTRAINT_NAME`

const schemaFingerprintReferentialSQL = `SELECT
  rc.CONSTRAINT_SCHEMA,
  kcu.TABLE_NAME,
  rc.CONSTRAINT_NAME,
  COALESCE(rc.UNIQUE_CONSTRAINT_SCHEMA, ''),
  COALESCE(rc.UNIQUE_CONSTRAINT_NAME, ''),
  rc.MATCH_OPTION,
  rc.UPDATE_RULE,
  rc.DELETE_RULE
FROM information_schema.referential_constraints rc
JOIN information_schema.key_column_usage kcu
  ON kcu.CONSTRAINT_SCHEMA = rc.CONSTRAINT_SCHEMA
 AND kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
WHERE rc.CONSTRAINT_SCHEMA = ?
GROUP BY
  rc.CONSTRAINT_SCHEMA,
  kcu.TABLE_NAME,
  rc.CONSTRAINT_NAME,
  rc.UNIQUE_CONSTRAINT_SCHEMA,
  rc.UNIQUE_CONSTRAINT_NAME,
  rc.MATCH_OPTION,
  rc.UPDATE_RULE,
  rc.DELETE_RULE
ORDER BY rc.CONSTRAINT_SCHEMA, kcu.TABLE_NAME, rc.CONSTRAINT_NAME`

type schemaFingerprintCollector struct {
	query    Queryer
	database string
}

type schemaFingerprintGroup struct {
	kind  string
	query string
	width int
}

func NewSchemaFingerprint(query Queryer, database string) collector.Collector {
	return &schemaFingerprintCollector{query: query, database: database}
}

func (c *schemaFingerprintCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.schema_fingerprint",
		Requires: []capability.Capability{"mysql.schema_fingerprint"},
		Produces: []signal.Key{"mysql.schema.structural_fingerprint"},
		Strategy: collector.StrategySnapshot,
	}
}

func (c *schemaFingerprintCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	groups := []schemaFingerprintGroup{
		{kind: "table", query: schemaFingerprintTablesSQL, width: 6},
		{kind: "column", query: schemaFingerprintColumnsSQL, width: 11},
		{kind: "index", query: schemaFingerprintIndexesSQL, width: 11},
		{kind: "constraint", query: schemaFingerprintConstraintsSQL, width: 9},
		{kind: "check", query: schemaFingerprintChecksSQL, width: 4},
		{kind: "referential", query: schemaFingerprintReferentialSQL, width: 8},
	}

	records := make([][]byte, 0, 128)
	totalBytes := 0
	for _, group := range groups {
		groupRecords, groupBytes, err := c.collectGroup(ctx, group, maxSchemaFingerprintTotalBytes-totalBytes)
		if err != nil {
			return nil, err
		}
		records = append(records, groupRecords...)
		totalBytes += groupBytes
	}

	fingerprint := hashSchemaRecords(records)
	observation := signal.Observation{
		Key:         "mysql.schema.structural_fingerprint",
		Object:      object.Ref{Kind: "mysql.schema", ID: c.database},
		Exactness:   signal.ExactnessScraped,
		Text:        &fingerprint,
		CollectedAt: req.CollectedAt,
		Sensitivity: signal.SensitivityMetadata,
		Source:      "information_schema:mysql-schema-fingerprint/v1",
	}
	return []signal.Observation{observation}, nil
}

func (c *schemaFingerprintCollector) collectGroup(ctx context.Context, group schemaFingerprintGroup, remainingBytes int) ([][]byte, int, error) {
	rows, err := c.query.QueryContext(ctx, group.query, c.database)
	if err != nil {
		return nil, 0, fmt.Errorf("collect mysql.schema_fingerprint %s metadata: %w", group.kind, err)
	}
	defer rows.Close()

	records := make([][]byte, 0)
	usedBytes := 0
	for rows.Next() {
		if len(records) >= maxSchemaFingerprintRowsPerGroup {
			return nil, 0, fmt.Errorf("collect mysql.schema_fingerprint %s metadata: row limit exceeded", group.kind)
		}
		fields := make([]string, group.width)
		dest := make([]any, group.width)
		for i := range fields {
			dest[i] = &fields[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, 0, fmt.Errorf("scan mysql.schema_fingerprint %s metadata: %w", group.kind, err)
		}
		for _, field := range fields {
			if len(field) > maxSchemaFingerprintFieldBytes {
				return nil, 0, fmt.Errorf("collect mysql.schema_fingerprint %s metadata: field too large", group.kind)
			}
		}
		record, err := encodeSchemaRecord(group.kind, fields)
		if err != nil {
			return nil, 0, fmt.Errorf("collect mysql.schema_fingerprint %s metadata: %w", group.kind, err)
		}
		if len(record) > remainingBytes-usedBytes {
			return nil, 0, fmt.Errorf("collect mysql.schema_fingerprint %s metadata: total metadata budget exceeded", group.kind)
		}
		records = append(records, record)
		usedBytes += len(record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate mysql.schema_fingerprint %s metadata: %w", group.kind, err)
	}
	return records, usedBytes, nil
}

func encodeSchemaRecord(kind string, fields []string) ([]byte, error) {
	var out bytes.Buffer
	writeLengthPrefixed(&out, []byte(kind))
	var count [4]byte
	binary.BigEndian.PutUint32(count[:], uint32(len(fields)))
	out.Write(count[:])
	for _, field := range fields {
		writeLengthPrefixed(&out, []byte(field))
		if out.Len() > maxSchemaFingerprintRecordBytes {
			return nil, fmt.Errorf("canonical record too large")
		}
	}
	return out.Bytes(), nil
}

func writeLengthPrefixed(out *bytes.Buffer, value []byte) {
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(value)))
	out.Write(size[:])
	out.Write(value)
}

func hashSchemaRecords(records [][]byte) string {
	sort.Slice(records, func(i, j int) bool { return bytes.Compare(records[i], records[j]) < 0 })
	hash := sha256.New()
	hash.Write([]byte("dbprobe:mysql-schema-fingerprint:v1\x00"))
	for _, record := range records {
		var size [4]byte
		binary.BigEndian.PutUint32(size[:], uint32(len(record)))
		hash.Write(size[:])
		hash.Write(record)
	}
	return schemaFingerprintVersion + ":sha256:" + hex.EncodeToString(hash.Sum(nil))
}
