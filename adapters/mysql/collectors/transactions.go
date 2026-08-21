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

const transactionSQL = `SELECT
  COALESCE(CAST(TRX_ID AS CHAR), ''),
  CAST(TRX_MYSQL_THREAD_ID AS CHAR),
  TRX_STATE,
  GREATEST(TIMESTAMPDIFF(SECOND, TRX_STARTED, CURRENT_TIMESTAMP), 0),
  TRX_ROWS_LOCKED,
  TRX_ROWS_MODIFIED,
  CASE WHEN TRX_WAIT_STARTED IS NULL THEN 0
       ELSE GREATEST(TIMESTAMPDIFF(SECOND, TRX_WAIT_STARTED, CURRENT_TIMESTAMP), 0)
  END AS lock_wait_seconds
FROM information_schema.innodb_trx
ORDER BY TRX_STARTED ASC
LIMIT ?`

type transactionCollector struct {
	query Queryer
	limit int
}

func NewTransactions(query Queryer, limit int) collector.Collector {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	return &transactionCollector{query: query, limit: limit}
}

func (c *transactionCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mysql.transactions",
		Requires: []capability.Capability{"activity.transactions"},
		Produces: []signal.Key{
			"mysql.transaction.age_seconds",
			"mysql.transaction.rows_locked",
			"mysql.transaction.rows_modified",
			"mysql.transaction.lock_wait_seconds",
			"mysql.transaction.state",
		},
		Strategy: collector.StrategySnapshot,
	}
}

func (c *transactionCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	rows, err := c.query.QueryContext(ctx, transactionSQL, c.limit)
	if err != nil {
		return nil, fmt.Errorf("collect mysql.transactions: %w", err)
	}
	defer rows.Close()

	observations := make([]signal.Observation, 0, c.limit*5)
	for rows.Next() {
		var trxID, threadID, state, ageRaw, lockedRaw, modifiedRaw, waitRaw string
		if err := rows.Scan(&trxID, &threadID, &state, &ageRaw, &lockedRaw, &modifiedRaw, &waitRaw); err != nil {
			return nil, fmt.Errorf("scan mysql.transactions: %w", err)
		}
		objectID := "thread:" + threadID
		if trxID != "" {
			objectID = "ephemeral:" + trxID
		}
		ref := object.Ref{Kind: "mysql.transaction", ID: objectID}

		age, err := strconv.ParseFloat(ageRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse mysql.transaction age: %w", err)
		}
		locked, err := strconv.ParseFloat(lockedRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse mysql.transaction rows locked: %w", err)
		}
		modified, err := strconv.ParseFloat(modifiedRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse mysql.transaction rows modified: %w", err)
		}
		waitSeconds, err := strconv.ParseFloat(waitRaw, 64)
		if err != nil {
			return nil, fmt.Errorf("parse mysql.transaction lock wait seconds: %w", err)
		}

		ageObservation := signal.NumberObservation("mysql.transaction.age_seconds", ref, age, signal.UnitSeconds, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt)
		lockedObservation := signal.NumberObservation("mysql.transaction.rows_locked", ref, locked, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt)
		modifiedObservation := signal.NumberObservation("mysql.transaction.rows_modified", ref, modified, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt)
		stateValue := state
		stateObservation := signal.Observation{
			Key:         "mysql.transaction.state",
			Object:      ref,
			Exactness:   signal.ExactnessScraped,
			Text:        &stateValue,
			CollectedAt: req.CollectedAt,
			Sensitivity: signal.SensitivityMetadata,
			Source:      "information_schema.innodb_trx",
		}
		for _, observation := range []*signal.Observation{&ageObservation, &lockedObservation, &modifiedObservation} {
			observation.Source = "information_schema.innodb_trx"
		}
		observations = append(observations, ageObservation, lockedObservation, modifiedObservation, stateObservation)
		if state == "LOCK WAIT" {
			waitObservation := signal.NumberObservation("mysql.transaction.lock_wait_seconds", ref, waitSeconds, signal.UnitSeconds, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt)
			waitObservation.Source = "information_schema.innodb_trx"
			observations = append(observations, waitObservation)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate mysql.transactions: %w", err)
	}
	return observations, nil
}
