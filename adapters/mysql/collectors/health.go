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

const snapshotStatusQuery = `SELECT VARIABLE_NAME, VARIABLE_VALUE
FROM performance_schema.global_status
WHERE VARIABLE_NAME IN ('Threads_connected', 'Threads_running', 'Uptime')
UNION ALL
SELECT VARIABLE_NAME, VARIABLE_VALUE
FROM performance_schema.global_variables
WHERE VARIABLE_NAME = 'max_connections'`

const counterStatusQuery = `SELECT VARIABLE_NAME, VARIABLE_VALUE
FROM performance_schema.global_status
WHERE VARIABLE_NAME IN (
  'Connections',
  'Created_tmp_disk_tables',
  'Innodb_buffer_pool_read_requests',
  'Innodb_buffer_pool_reads',
  'Innodb_row_lock_waits',
  'Innodb_log_waits',
  'Com_commit',
  'Com_rollback'
)`

type statusCollector struct {
	query      Queryer
	instanceID string
	descriptor collector.Descriptor
	sql        string
	exactness  signal.Exactness
	mapping    map[string]signal.Key
}

func NewHealth(query Queryer, instanceID string) []collector.Collector {
	return []collector.Collector{
		&statusCollector{
			query:      query,
			instanceID: instanceID,
			descriptor: collector.Descriptor{
				ID:       "mysql.health.snapshot",
				Requires: []capability.Capability{"mysql.performance_schema"},
				Produces: []signal.Key{
					"core.connections.used",
					"core.connections.limit",
					"mysql.threads.running",
					"mysql.server.uptime_seconds",
				},
				Strategy: collector.StrategySnapshot,
			},
			sql:       snapshotStatusQuery,
			exactness: signal.ExactnessScraped,
			mapping: map[string]signal.Key{
				"threads_connected": "core.connections.used",
				"threads_running":   "mysql.threads.running",
				"uptime":            "mysql.server.uptime_seconds",
				"max_connections":   "core.connections.limit",
			},
		},
		&statusCollector{
			query:      query,
			instanceID: instanceID,
			descriptor: collector.Descriptor{
				ID:       "mysql.health.counters",
				Requires: []capability.Capability{"mysql.performance_schema"},
				Produces: []signal.Key{
					"mysql.connections.total",
					"mysql.temp.disk_tables",
					"mysql.innodb.buffer_pool.read_requests",
					"mysql.innodb.buffer_pool.reads",
					"mysql.innodb.row_lock_waits",
					"mysql.innodb.log_waits",
					"mysql.statements.commit",
					"mysql.statements.rollback",
				},
				Strategy: collector.StrategyCounter,
			},
			sql:       counterStatusQuery,
			exactness: signal.ExactnessCumulative,
			mapping: map[string]signal.Key{
				"connections":                      "mysql.connections.total",
				"created_tmp_disk_tables":          "mysql.temp.disk_tables",
				"innodb_buffer_pool_read_requests": "mysql.innodb.buffer_pool.read_requests",
				"innodb_buffer_pool_reads":         "mysql.innodb.buffer_pool.reads",
				"innodb_row_lock_waits":            "mysql.innodb.row_lock_waits",
				"innodb_log_waits":                 "mysql.innodb.log_waits",
				"com_commit":                       "mysql.statements.commit",
				"com_rollback":                     "mysql.statements.rollback",
			},
		},
	}
}

func (c *statusCollector) Descriptor() collector.Descriptor { return c.descriptor }

func (c *statusCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	rows, err := c.query.QueryContext(ctx, c.sql)
	if err != nil {
		return nil, fmt.Errorf("collect %s: %w", c.descriptor.ID, err)
	}
	defer rows.Close()

	observations := make([]signal.Observation, 0, len(c.mapping))
	for rows.Next() {
		var name, raw string
		if err := rows.Scan(&name, &raw); err != nil {
			return nil, fmt.Errorf("scan %s: %w", c.descriptor.ID, err)
		}
		key, ok := c.mapping[strings.ToLower(name)]
		if !ok {
			continue
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse %s metric %q: %w", c.descriptor.ID, name, err)
		}
		unit := signal.UnitCount
		if key == "mysql.server.uptime_seconds" {
			unit = signal.UnitSeconds
		}
		observation := signal.NumberObservation(
			key,
			object.Ref{Kind: "mysql.instance", ID: c.instanceID},
			value,
			unit,
			c.exactness,
			signal.SensitivityMetadata,
			req.CollectedAt,
		)
		observation.Source = "performance_schema"
		observations = append(observations, observation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s: %w", c.descriptor.ID, err)
	}
	return observations, nil
}
