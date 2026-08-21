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

const purgeHistorySQL = `SELECT CAST(COUNT AS CHAR)
FROM INFORMATION_SCHEMA.INNODB_METRICS
WHERE NAME = 'trx_rseg_history_len'
LIMIT 1`

type purgeHistoryCollector struct {
	query      Queryer
	instanceID string
}

func NewPurgeHistory(query Queryer, instanceID string) collector.Collector {
	return &purgeHistoryCollector{query: query, instanceID: instanceID}
}

func (c *purgeHistoryCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.innodb.purge-history",
		Requires: []capability.Capability{"mysql.innodb_metrics"},
		Produces: []signal.Key{"mysql.innodb.history_list_length"},
		Strategy: collector.StrategySnapshot,
	}
}

func (c *purgeHistoryCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	rows, err := c.query.QueryContext(ctx, purgeHistorySQL)
	if err != nil {
		return nil, fmt.Errorf("collect mysql.innodb.purge-history: %w", err)
	}
	defer rows.Close()

	observations := []signal.Observation{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan mysql.innodb.purge-history: %w", err)
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse mysql.innodb history list length: %w", err)
		}
		observation := signal.NumberObservation("mysql.innodb.history_list_length", object.Ref{Kind: "mysql.instance", ID: c.instanceID}, value, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt)
		observation.Source = "information_schema.innodb_metrics"
		observations = append(observations, observation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql.innodb.purge-history: %w", err)
	}
	return observations, nil
}
