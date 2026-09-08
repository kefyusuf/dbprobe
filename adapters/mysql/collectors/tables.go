package collectors

import (
	"context"
	"fmt"
	"strconv"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

const tableScanSQL = `SELECT
  t.TABLE_SCHEMA,
  t.TABLE_NAME,
  COALESCE(t.TABLE_ROWS, 0),
  COALESCE(p.COUNT_READ, 0)
FROM information_schema.tables t
LEFT JOIN performance_schema.table_io_waits_summary_by_index_usage p
  ON p.OBJECT_SCHEMA = t.TABLE_SCHEMA
 AND p.OBJECT_NAME = t.TABLE_NAME
 AND p.INDEX_NAME IS NULL
WHERE t.TABLE_SCHEMA = ?
  AND t.TABLE_TYPE = 'BASE TABLE'
ORDER BY COALESCE(p.COUNT_READ, 0) DESC
LIMIT ?`

type tableCollector struct {
	query    Queryer
	database string
	limit    int
}

func NewTables(query Queryer, database string, limit int) collector.Collector {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	return &tableCollector{query: query, database: database, limit: limit}
}

func (c *tableCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.tables",
		Requires: []capability.Capability{"schema.objects", "schema.indexes"},
		Produces: []signal.Key{
			"mysql.table.estimated_rows",
			"mysql.table.full_scan_rows",
		},
		Strategy: collector.StrategyCounter,
	}
}

func (c *tableCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	rows, err := c.query.QueryContext(ctx, tableScanSQL, c.database, c.limit)
	if err != nil {
		return nil, fmt.Errorf("collect mysql.tables: %w", err)
	}
	defer rows.Close()

	observations := make([]signal.Observation, 0, c.limit*2)
	for rows.Next() {
		var schemaName, tableName, estimatedRowsRaw, fullScanRowsRaw string
		if err := rows.Scan(&schemaName, &tableName, &estimatedRowsRaw, &fullScanRowsRaw); err != nil {
			return nil, fmt.Errorf("scan mysql.tables: %w", err)
		}
		estimatedRows, err := strconv.ParseFloat(estimatedRowsRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse mysql.table estimated rows: %w", err)
		}
		fullScanRows, err := strconv.ParseFloat(fullScanRowsRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse mysql.table full scan rows: %w", err)
		}
		ref := object.Ref{Kind: "mysql.table", ID: schemaName + "." + tableName}

		estimate := signal.NumberObservation("mysql.table.estimated_rows", ref, estimatedRows, signal.UnitCount, signal.ExactnessEstimated, signal.SensitivityMetadata, req.CollectedAt)
		estimate.Source = "information_schema.tables"
		scans := signal.NumberObservation("mysql.table.full_scan_rows", ref, fullScanRows, signal.UnitCount, signal.ExactnessCumulative, signal.SensitivityMetadata, req.CollectedAt)
		scans.Source = "performance_schema.table_io_waits_summary_by_index_usage"
		observations = append(observations, estimate, scans)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql.tables: %w", err)
	}
	return observations, nil
}
