package sqlite

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type metricKind string

const (
	metricKindObservation metricKind = "observation"
	metricKindDelta       metricKind = "delta"
)

type trendMetric struct {
	SnapshotID    string
	MetricKind    metricKind
	SignalKey     signal.Key
	ObjectKind    string
	ObjectID      string
	NumericValue  float64
	Unit          signal.Unit
	Exactness     signal.Exactness
	RatePerSecond *float64
	WindowSeconds *float64
}

func encodeSnapshot(snapshot temporal.Snapshot) ([]byte, error) {
	normalized, err := validateSnapshot(snapshot)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("encode SQLite snapshot payload: %w", err)
	}
	return payload, nil
}

func decodeSnapshot(payload []byte) (temporal.Snapshot, error) {
	if len(payload) == 0 {
		return temporal.Snapshot{}, fmt.Errorf("SQLite snapshot payload is empty")
	}
	var snapshot temporal.Snapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return temporal.Snapshot{}, fmt.Errorf("decode SQLite snapshot payload: %w", err)
	}
	return validateSnapshot(snapshot)
}

func validateSnapshot(snapshot temporal.Snapshot) (temporal.Snapshot, error) {
	if snapshot.SchemaVersion != temporal.SnapshotSchemaVersion {
		return temporal.Snapshot{}, fmt.Errorf("unsupported snapshot schema version %q", snapshot.SchemaVersion)
	}
	normalized, err := temporal.NewSnapshot(temporal.SnapshotInput{
		TargetFingerprint: snapshot.TargetFingerprint,
		Engine:            snapshot.Engine,
		AdapterID:         snapshot.AdapterID,
		AdapterVersion:    snapshot.AdapterVersion,
		CollectedAt:       snapshot.CollectedAt,
		Capabilities:      snapshot.Capabilities,
		Observations:      snapshot.Observations,
		Deltas:            snapshot.Deltas,
		Findings:          snapshot.Findings,
	})
	if err != nil {
		return temporal.Snapshot{}, fmt.Errorf("validate SQLite snapshot payload: %w", err)
	}
	if normalized.ID != snapshot.ID {
		return temporal.Snapshot{}, fmt.Errorf("SQLite snapshot payload ID does not match target/time identity")
	}
	return normalized, nil
}

func extractTrendMetrics(snapshot temporal.Snapshot) []trendMetric {
	rows := make([]trendMetric, 0, len(snapshot.Observations)+len(snapshot.Deltas))
	for _, observation := range snapshot.Observations {
		if observation.Number == nil {
			continue
		}
		rows = append(rows, trendMetric{
			SnapshotID:   snapshot.ID,
			MetricKind:   metricKindObservation,
			SignalKey:    observation.Key,
			ObjectKind:   observation.Object.Kind,
			ObjectID:     observation.Object.ID,
			NumericValue: *observation.Number,
			Unit:         observation.Unit,
			Exactness:    observation.Exactness,
		})
	}
	for _, delta := range snapshot.Deltas {
		rate := delta.RatePerSecond
		window := delta.WindowSeconds
		rows = append(rows, trendMetric{
			SnapshotID:    snapshot.ID,
			MetricKind:    metricKindDelta,
			SignalKey:     delta.Key,
			ObjectKind:    delta.Object.Kind,
			ObjectID:      delta.Object.ID,
			NumericValue:  delta.Delta,
			Unit:          delta.Unit,
			Exactness:     delta.Exactness,
			RatePerSecond: &rate,
			WindowSeconds: &window,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		if left.MetricKind != right.MetricKind {
			return left.MetricKind == metricKindObservation
		}
		if left.SignalKey != right.SignalKey {
			return left.SignalKey < right.SignalKey
		}
		if left.ObjectKind != right.ObjectKind {
			return left.ObjectKind < right.ObjectKind
		}
		return left.ObjectID < right.ObjectID
	})
	return rows
}
