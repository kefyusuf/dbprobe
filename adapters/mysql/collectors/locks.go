package collectors

import (
	"context"
	"fmt"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

const lockWaitSQL = `SELECT
  CAST(w.REQUESTING_ENGINE_TRANSACTION_ID AS CHAR),
  CAST(w.BLOCKING_ENGINE_TRANSACTION_ID AS CHAR),
  COALESCE(l.OBJECT_SCHEMA, ''),
  COALESCE(l.OBJECT_NAME, ''),
  COALESCE(l.INDEX_NAME, '')
FROM performance_schema.data_lock_waits w
LEFT JOIN performance_schema.data_locks l
  ON l.ENGINE = w.ENGINE
 AND l.ENGINE_LOCK_ID = w.REQUESTING_ENGINE_LOCK_ID
WHERE w.ENGINE = 'INNODB'
LIMIT ?`

type lockCollector struct {
	query      Queryer
	instanceID string
	limit      int
}

func NewLocks(query Queryer, instanceID string, limit int) collector.Collector {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	return &lockCollector{query: query, instanceID: instanceID, limit: limit}
}

func (c *lockCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.locks",
		Requires: []capability.Capability{"locking.wait_graph"},
		Produces: []signal.Key{
			"mysql.lock_wait.count",
			"mysql.lock_wait.edge",
			"mysql.lock_wait.index",
		},
		Strategy: collector.StrategySnapshot,
	}
}

func (c *lockCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	rows, err := c.query.QueryContext(ctx, lockWaitSQL, c.limit)
	if err != nil {
		return nil, fmt.Errorf("collect mysql.locks: %w", err)
	}
	defer rows.Close()

	observations := make([]signal.Observation, 0, c.limit*3)
	for rows.Next() {
		var requesting, blocking, schemaName, tableName, indexName string
		if err := rows.Scan(&requesting, &blocking, &schemaName, &tableName, &indexName); err != nil {
			return nil, fmt.Errorf("scan mysql.locks: %w", err)
		}
		ref := object.Ref{Kind: "mysql.instance", ID: c.instanceID}
		if schemaName != "" && tableName != "" {
			ref = object.Ref{Kind: "mysql.table", ID: schemaName + "." + tableName}
		}

		count := signal.NumberObservation("mysql.lock_wait.count", ref, 1, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt)
		count.Source = "performance_schema.data_lock_waits"
		edgeValue := requesting + "->" + blocking
		edge := signal.Observation{
			Key:         "mysql.lock_wait.edge",
			Object:      ref,
			Exactness:   signal.ExactnessScraped,
			Text:        &edgeValue,
			CollectedAt: req.CollectedAt,
			Sensitivity: signal.SensitivityMetadata,
			Source:      "performance_schema.data_lock_waits",
		}
		observations = append(observations, count, edge)

		if indexName != "" {
			indexValue := indexName
			observations = append(observations, signal.Observation{
				Key:         "mysql.lock_wait.index",
				Object:      ref,
				Exactness:   signal.ExactnessScraped,
				Text:        &indexValue,
				CollectedAt: req.CollectedAt,
				Sensitivity: signal.SensitivityMetadata,
				Source:      "performance_schema.data_locks",
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql.locks: %w", err)
	}
	return observations, nil
}
