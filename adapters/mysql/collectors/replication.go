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

const replicationSQL = `SELECT component, channel_name, service_state, error_number
FROM (
  SELECT
    'receiver' AS component,
    CHANNEL_NAME AS channel_name,
    SERVICE_STATE AS service_state,
    CAST(LAST_ERROR_NUMBER AS CHAR) AS error_number
  FROM performance_schema.replication_connection_status

  UNION ALL

  SELECT
    'applier' AS component,
    CHANNEL_NAME AS channel_name,
    SERVICE_STATE AS service_state,
    '0' AS error_number
  FROM performance_schema.replication_applier_status

  UNION ALL

  SELECT
    'coordinator' AS component,
    CHANNEL_NAME AS channel_name,
    SERVICE_STATE AS service_state,
    CAST(LAST_ERROR_NUMBER AS CHAR) AS error_number
  FROM performance_schema.replication_applier_status_by_coordinator

  UNION ALL

  SELECT
    CONCAT('worker:', WORKER_ID) AS component,
    CHANNEL_NAME AS channel_name,
    SERVICE_STATE AS service_state,
    CAST(LAST_ERROR_NUMBER AS CHAR) AS error_number
  FROM performance_schema.replication_applier_status_by_worker
) AS replication_state
ORDER BY channel_name, component
LIMIT ?`

type replicationCollector struct {
	query Queryer
	limit int
}

func NewReplication(query Queryer, limit int) collector.Collector {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	return &replicationCollector{query: query, limit: limit}
}

func (c *replicationCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.replication",
		Requires: []capability.Capability{"replication.status"},
		Produces: []signal.Key{
			"mysql.replication.receiver_on",
			"mysql.replication.receiver_state",
			"mysql.replication.applier_on",
			"mysql.replication.applier_state",
			"mysql.replication.worker_on",
			"mysql.replication.worker_state",
			"mysql.replication.error_code",
		},
		Strategy: collector.StrategySnapshot,
	}
}

func (c *replicationCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	rows, err := c.query.QueryContext(ctx, replicationSQL, c.limit)
	if err != nil {
		return nil, fmt.Errorf("collect mysql.replication: %w", err)
	}
	defer rows.Close()

	observations := make([]signal.Observation, 0, c.limit*3)
	for rows.Next() {
		var component, channel, state, errorRaw string
		if err := rows.Scan(&component, &channel, &state, &errorRaw); err != nil {
			return nil, fmt.Errorf("scan mysql.replication: %w", err)
		}
		if channel == "" {
			channel = "default"
		}
		channelRef := object.Ref{Kind: "mysql.replication_channel", ID: channel}
		componentRef := channelRef
		if component != "receiver" && component != "applier" {
			componentRef = object.Ref{Kind: "mysql.replication_worker", ID: channel + ":" + component}
		}

		stateKey := signal.Key("mysql.replication.worker_state")
		onKey := signal.Key("mysql.replication.worker_on")
		switch component {
		case "receiver":
			stateKey = "mysql.replication.receiver_state"
			onKey = "mysql.replication.receiver_on"
		case "applier":
			stateKey = "mysql.replication.applier_state"
			onKey = "mysql.replication.applier_on"
		}

		stateValue := strings.ToUpper(state)
		stateObservation := signal.Observation{
			Key:         stateKey,
			Object:      componentRef,
			Exactness:   signal.ExactnessScraped,
			Text:        &stateValue,
			CollectedAt: req.CollectedAt,
			Sensitivity: signal.SensitivityMetadata,
			Source:      "performance_schema.replication_*",
		}
		onObservation := boolObservation(onKey, componentRef, stateValue == "ON", req.CollectedAt, "performance_schema.replication_*")
		observations = append(observations, stateObservation, onObservation)

		errorCode, err := strconv.ParseFloat(errorRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse mysql.replication error number: %w", err)
		}
		if errorCode > 0 {
			errorObservation := signal.NumberObservation("mysql.replication.error_code", componentRef, errorCode, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt)
			errorObservation.Source = "performance_schema.replication_*"
			observations = append(observations, errorObservation)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql.replication: %w", err)
	}
	return observations, nil
}
