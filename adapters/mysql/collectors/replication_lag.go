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

const replicationActiveLagSQL = `SELECT
  w.CHANNEL_NAME,
  CAST(GREATEST(
    MAX(TIMESTAMPDIFF(MICROSECOND, w.APPLYING_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP, CURRENT_TIMESTAMP(6))) / 1000000.0
      - COALESCE(MAX(c.DESIRED_DELAY), 0),
    0
  ) AS CHAR) AS active_apply_lag_seconds,
  CAST(COALESCE(MAX(c.DESIRED_DELAY), 0) AS CHAR) AS desired_delay_seconds
FROM performance_schema.replication_applier_status_by_worker w
LEFT JOIN performance_schema.replication_applier_configuration c
  ON c.CHANNEL_NAME = w.CHANNEL_NAME
WHERE w.APPLYING_TRANSACTION IS NOT NULL
  AND w.APPLYING_TRANSACTION <> ''
  AND w.APPLYING_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP IS NOT NULL
  AND w.APPLYING_TRANSACTION_ORIGINAL_COMMIT_TIMESTAMP > '1970-01-01 00:00:00'
GROUP BY w.CHANNEL_NAME
ORDER BY active_apply_lag_seconds DESC
LIMIT ?`

type replicationLagCollector struct {
	query Queryer
	limit int
}

func NewReplicationLag(query Queryer, limit int) collector.Collector {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	return &replicationLagCollector{query: query, limit: limit}
}

func (c *replicationLagCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.replication.active-lag",
		Requires: []capability.Capability{"replication.status"},
		Produces: []signal.Key{"mysql.replication.active_apply_lag_seconds", "mysql.replication.desired_delay_seconds"},
		Strategy: collector.StrategySnapshot,
	}
}

func (c *replicationLagCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	rows, err := c.query.QueryContext(ctx, replicationActiveLagSQL, c.limit)
	if err != nil {
		return nil, fmt.Errorf("collect mysql.replication.active-lag: %w", err)
	}
	defer rows.Close()

	observations := make([]signal.Observation, 0, c.limit*2)
	for rows.Next() {
		var channel, lagRaw, desiredDelayRaw string
		if err := rows.Scan(&channel, &lagRaw, &desiredDelayRaw); err != nil {
			return nil, fmt.Errorf("scan mysql.replication.active-lag: %w", err)
		}
		if channel == "" {
			channel = "default"
		}
		lag, err := strconv.ParseFloat(lagRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse mysql.replication active lag: %w", err)
		}
		desiredDelay, err := strconv.ParseFloat(desiredDelayRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse mysql.replication desired delay: %w", err)
		}
		ref := object.Ref{Kind: "mysql.replication_channel", ID: channel}
		lagObs := signal.NumberObservation("mysql.replication.active_apply_lag_seconds", ref, lag, signal.UnitSeconds, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt)
		lagObs.Source = "performance_schema.replication_applier_status_by_worker"
		delayObs := signal.NumberObservation("mysql.replication.desired_delay_seconds", ref, desiredDelay, signal.UnitSeconds, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt)
		delayObs.Source = "performance_schema.replication_applier_configuration"
		observations = append(observations, lagObs, delayObs)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql.replication.active-lag: %w", err)
	}
	return observations, nil
}
