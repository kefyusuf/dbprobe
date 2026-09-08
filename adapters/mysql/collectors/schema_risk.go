package collectors

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

const primaryKeySQL = `SELECT
  t.TABLE_SCHEMA,
  t.TABLE_NAME,
  CASE WHEN EXISTS (
    SELECT 1
    FROM information_schema.statistics s
    WHERE s.TABLE_SCHEMA = t.TABLE_SCHEMA
      AND s.TABLE_NAME = t.TABLE_NAME
      AND s.INDEX_NAME = 'PRIMARY'
  ) THEN '1' ELSE '0' END AS primary_key_present
FROM information_schema.tables t
WHERE t.TABLE_SCHEMA = ?
  AND t.TABLE_TYPE = 'BASE TABLE'
ORDER BY t.TABLE_NAME
LIMIT ?`

const autoIncrementSQL = `SELECT
  c.TABLE_SCHEMA,
  c.TABLE_NAME,
  c.COLUMN_NAME,
  LOWER(c.DATA_TYPE),
  LOWER(c.COLUMN_TYPE),
  CAST(COALESCE(t.AUTO_INCREMENT, 0) AS CHAR)
FROM information_schema.columns c
JOIN information_schema.tables t
  ON t.TABLE_SCHEMA = c.TABLE_SCHEMA
 AND t.TABLE_NAME = c.TABLE_NAME
WHERE c.TABLE_SCHEMA = ?
  AND c.EXTRA LIKE '%auto_increment%'
  AND t.TABLE_TYPE = 'BASE TABLE'
ORDER BY c.TABLE_NAME, c.ORDINAL_POSITION
LIMIT ?`

type primaryKeyCollector struct {
	query    Queryer
	database string
	limit    int
}

type autoIncrementCollector struct {
	query    Queryer
	database string
	limit    int
}

func NewSchemaRisk(query Queryer, database string, limit int) []collector.Collector {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	return []collector.Collector{
		&primaryKeyCollector{query: query, database: database, limit: limit},
		&autoIncrementCollector{query: query, database: database, limit: limit},
	}
}

func (c *primaryKeyCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.schema.primary_keys",
		Requires: []capability.Capability{"schema.objects"},
		Produces: []signal.Key{"mysql.table.primary_key_present"},
		Strategy: collector.StrategySnapshot,
	}
}

func (c *primaryKeyCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	rows, err := c.query.QueryContext(ctx, primaryKeySQL, c.database, c.limit+1)
	if err != nil {
		return nil, fmt.Errorf("collect mysql.schema.primary_keys: %w", err)
	}
	defer rows.Close()

	observations := make([]signal.Observation, 0, c.limit)
	truncated := false
	for rows.Next() {
		var schemaName, tableName, presentRaw string
		if err := rows.Scan(&schemaName, &tableName, &presentRaw); err != nil {
			return nil, fmt.Errorf("scan mysql.schema.primary_keys: %w", err)
		}
		if len(observations) >= c.limit {
			truncated = true
			break
		}
		present := presentRaw == "1"
		ref := object.Ref{Kind: "mysql.table", ID: schemaName + "." + tableName}
		observations = append(observations, boolObservation("mysql.table.primary_key_present", ref, present, req.CollectedAt, "information_schema.statistics"))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql.schema.primary_keys: %w", err)
	}
	if truncated {
		return observations, fmt.Errorf("primary-key scan truncated after %d tables", c.limit)
	}
	return observations, nil
}

func (c *autoIncrementCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.schema.auto_increment",
		Requires: []capability.Capability{"schema.objects"},
		Produces: []signal.Key{
			"mysql.auto_increment.next_value",
			"mysql.auto_increment.max_value",
			"mysql.auto_increment.utilization_ratio",
		},
		Strategy: collector.StrategySnapshot,
	}
}

func (c *autoIncrementCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	rows, err := c.query.QueryContext(ctx, autoIncrementSQL, c.database, c.limit+1)
	if err != nil {
		return nil, fmt.Errorf("collect mysql.schema.auto_increment: %w", err)
	}
	defer rows.Close()

	observations := make([]signal.Observation, 0, c.limit*3)
	rowsSeen := 0
	truncated := false
	for rows.Next() {
		var schemaName, tableName, columnName, dataType, columnType, nextRaw string
		if err := rows.Scan(&schemaName, &tableName, &columnName, &dataType, &columnType, &nextRaw); err != nil {
			return nil, fmt.Errorf("scan mysql.schema.auto_increment: %w", err)
		}
		if rowsSeen >= c.limit {
			truncated = true
			break
		}
		rowsSeen++
		maxValue, ok := autoIncrementMax(dataType, columnType)
		if !ok || maxValue <= 0 {
			continue
		}
		nextValue, err := strconv.ParseFloat(nextRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse mysql auto_increment next value: %w", err)
		}
		if nextValue < 0 {
			nextValue = 0
		}
		ratio := nextValue / maxValue
		if ratio < 0 {
			ratio = 0
		}
		ref := object.Ref{Kind: "mysql.column", ID: schemaName + "." + tableName + "." + columnName}
		nextObservation := signal.NumberObservation("mysql.auto_increment.next_value", ref, nextValue, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt)
		maxObservation := signal.NumberObservation("mysql.auto_increment.max_value", ref, maxValue, signal.UnitCount, signal.ExactnessEstimated, signal.SensitivityMetadata, req.CollectedAt)
		ratioObservation := signal.NumberObservation("mysql.auto_increment.utilization_ratio", ref, ratio, signal.UnitRatio, signal.ExactnessEstimated, signal.SensitivityMetadata, req.CollectedAt)
		for _, observation := range []*signal.Observation{&nextObservation, &maxObservation, &ratioObservation} {
			observation.Source = "information_schema.columns"
		}
		observations = append(observations, nextObservation, maxObservation, ratioObservation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql.schema.auto_increment: %w", err)
	}
	if truncated {
		return observations, fmt.Errorf("auto-increment scan truncated after %d columns", c.limit)
	}
	return observations, nil
}

func autoIncrementMax(dataType, columnType string) (float64, bool) {
	unsigned := strings.Contains(strings.ToLower(columnType), "unsigned")
	switch strings.ToLower(dataType) {
	case "tinyint":
		if unsigned {
			return 255, true
		}
		return 127, true
	case "smallint":
		if unsigned {
			return 65535, true
		}
		return 32767, true
	case "mediumint":
		if unsigned {
			return 16777215, true
		}
		return 8388607, true
	case "int", "integer":
		if unsigned {
			return 4294967295, true
		}
		return 2147483647, true
	case "bigint":
		if unsigned {
			return 18446744073709551615, true
		}
		return 9223372036854775807, true
	default:
		return 0, false
	}
}
