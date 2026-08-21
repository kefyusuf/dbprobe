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

const indexUsageSQL = `SELECT
  s.TABLE_SCHEMA,
  s.TABLE_NAME,
  s.INDEX_NAME,
  MIN(s.NON_UNIQUE),
  GROUP_CONCAT(COALESCE(s.COLUMN_NAME, s.EXPRESSION, '?') ORDER BY s.SEQ_IN_INDEX SEPARATOR ','),
  COALESCE(MAX(p.COUNT_READ), 0)
FROM information_schema.statistics s
LEFT JOIN performance_schema.table_io_waits_summary_by_index_usage p
  ON p.OBJECT_SCHEMA = s.TABLE_SCHEMA
 AND p.OBJECT_NAME = s.TABLE_NAME
 AND p.INDEX_NAME = s.INDEX_NAME
WHERE s.TABLE_SCHEMA = ?
GROUP BY s.TABLE_SCHEMA, s.TABLE_NAME, s.INDEX_NAME
ORDER BY s.TABLE_NAME, s.INDEX_NAME
LIMIT ?`

type indexCollector struct {
	query    Queryer
	database string
	limit    int
}

func NewIndexes(query Queryer, database string, limit int) collector.Collector {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	return &indexCollector{query: query, database: database, limit: limit}
}

func (c *indexCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.indexes",
		Requires: []capability.Capability{"schema.indexes"},
		Produces: []signal.Key{
			"mysql.index.reads",
			"mysql.index.columns",
			"mysql.index.unique",
			"mysql.index.primary",
		},
		Strategy: collector.StrategyCounter,
	}
}

func (c *indexCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	rows, err := c.query.QueryContext(ctx, indexUsageSQL, c.database, c.limit)
	if err != nil {
		return nil, fmt.Errorf("collect mysql.indexes: %w", err)
	}
	defer rows.Close()

	observations := make([]signal.Observation, 0, c.limit*4)
	for rows.Next() {
		var schemaName, tableName, indexName, nonUniqueRaw, columns, readsRaw string
		if err := rows.Scan(&schemaName, &tableName, &indexName, &nonUniqueRaw, &columns, &readsRaw); err != nil {
			return nil, fmt.Errorf("scan mysql.indexes: %w", err)
		}
		nonUnique, err := strconv.Atoi(nonUniqueRaw)
		if err != nil {
			return nil, fmt.Errorf("parse mysql.index unique flag: %w", err)
		}
		reads, err := strconv.ParseFloat(readsRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse mysql.index reads: %w", err)
		}
		ref := object.Ref{Kind: "mysql.index", ID: schemaName + "." + tableName + "." + indexName}

		readObservation := signal.NumberObservation("mysql.index.reads", ref, reads, signal.UnitCount, signal.ExactnessCumulative, signal.SensitivityMetadata, req.CollectedAt)
		readObservation.Source = "performance_schema.table_io_waits_summary_by_index_usage"
		observations = append(observations, readObservation)

		columnText := columns
		observations = append(observations, signal.Observation{
			Key:         "mysql.index.columns",
			Object:      ref,
			Exactness:   signal.ExactnessScraped,
			Text:        &columnText,
			CollectedAt: req.CollectedAt,
			Sensitivity: signal.SensitivityMetadata,
			Source:      "information_schema.statistics",
		})

		unique := nonUnique == 0
		primary := indexName == "PRIMARY"
		observations = append(observations,
			boolObservation("mysql.index.unique", ref, unique, req.CollectedAt, "information_schema.statistics"),
			boolObservation("mysql.index.primary", ref, primary, req.CollectedAt, "information_schema.statistics"),
		)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql.indexes: %w", err)
	}
	return observations, nil
}

func boolObservation(key signal.Key, ref object.Ref, value bool, at time.Time, source string) signal.Observation {
	return signal.Observation{
		Key:         key,
		Object:      ref,
		Exactness:   signal.ExactnessScraped,
		Boolean:     &value,
		CollectedAt: at,
		Sensitivity: signal.SensitivityMetadata,
		Source:      source,
	}
}
