package sqlite

import (
	"bytes"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func codecSnapshot(t *testing.T) temporal.Snapshot {
	t.Helper()
	at := time.Unix(100, 123).UTC()
	ref := object.Ref{Kind: "mysql.instance", ID: "one"}
	queryRef := object.Ref{Kind: "mysql.query", ID: "q1"}
	snapshot, err := temporal.NewSnapshot(temporal.SnapshotInput{
		TargetFingerprint: "target-1",
		Engine:            "mysql",
		AdapterID:         "mysql",
		AdapterVersion:    "0.1.0",
		CollectedAt:       at,
		Observations: []signal.Observation{
			signal.NumberObservation("core.connections.used", ref, 12, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, at),
			{Key: "mysql.query.digest_text", Object: queryRef, Text: stringPtr("SELECT * FROM t WHERE id=?"), Exactness: signal.ExactnessScraped, Sensitivity: signal.SensitivityQueryShape, CollectedAt: at},
		},
		Deltas: []signal.Delta{
			{Key: "core.query.calls", Object: queryRef, Unit: signal.UnitCount, Delta: 20, RatePerSecond: 2, WindowSeconds: 10, Exactness: signal.ExactnessSampled},
			{Key: "mysql.query.total_latency_ms", Object: queryRef, Unit: signal.UnitMilliseconds, Delta: 500, RatePerSecond: 50, WindowSeconds: 10, Exactness: signal.ExactnessSampled},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func stringPtr(v string) *string { return &v }

func TestSnapshotCodecRoundTripAndRejectsTamperedID(t *testing.T) {
	snapshot := codecSnapshot(t)
	payload, err := encodeSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	got, err := decodeSnapshot(payload)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != snapshot.ID || got.SchemaVersion != temporal.SnapshotSchemaVersion || got.TargetFingerprint != snapshot.TargetFingerprint {
		t.Fatalf("round trip=%#v", got)
	}
	if len(got.Observations) != 2 || got.Observations[1].Sensitivity != signal.SensitivityQueryShape {
		t.Fatalf("observations=%#v", got.Observations)
	}

	tampered := bytes.Replace(payload, []byte(snapshot.ID), []byte("00000000000000000000000000000000"), 1)
	if _, err := decodeSnapshot(tampered); err == nil {
		t.Fatal("tampered snapshot ID must be rejected")
	}
}

func TestSnapshotCodecReappliesSensitiveObservationFiltering(t *testing.T) {
	snapshot := codecSnapshot(t)
	secret := "SELECT * FROM users WHERE email='secret@example.test'"
	snapshot.Observations = append(snapshot.Observations, signal.Observation{
		Key:         "mysql.query.raw_text",
		Object:      object.Ref{Kind: "mysql.query", ID: "raw"},
		Text:        &secret,
		Exactness:   signal.ExactnessScraped,
		Sensitivity: signal.SensitivityQueryText,
		CollectedAt: snapshot.CollectedAt,
	})
	payload, err := encodeSnapshot(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(payload, []byte("secret@example.test")) {
		t.Fatalf("encoded snapshot leaked query text: %s", payload)
	}
	decoded, err := decodeSnapshot(payload)
	if err != nil {
		t.Fatal(err)
	}
	for _, observation := range decoded.Observations {
		if observation.Sensitivity == signal.SensitivityQueryText {
			t.Fatalf("decoded snapshot retained query text observation: %#v", observation)
		}
	}
}

func TestTrendRowsDistinguishObservationsAndDeltasDeterministically(t *testing.T) {
	snapshot := codecSnapshot(t)
	rows := extractTrendMetrics(snapshot)
	if len(rows) != 3 {
		t.Fatalf("rows=%#v", rows)
	}

	if rows[0].MetricKind != metricKindObservation || rows[0].SignalKey != "core.connections.used" || rows[0].RatePerSecond != nil || rows[0].WindowSeconds != nil {
		t.Fatalf("observation=%#v", rows[0])
	}
	if rows[1].MetricKind != metricKindDelta || rows[1].SignalKey != "core.query.calls" || rows[1].RatePerSecond == nil || *rows[1].RatePerSecond != 2 || rows[1].WindowSeconds == nil || *rows[1].WindowSeconds != 10 {
		t.Fatalf("first delta=%#v", rows[1])
	}
	if rows[2].MetricKind != metricKindDelta || rows[2].SignalKey != "mysql.query.total_latency_ms" {
		t.Fatalf("second delta=%#v", rows[2])
	}
	for _, row := range rows {
		if row.SnapshotID != snapshot.ID {
			t.Fatalf("snapshot id=%q", row.SnapshotID)
		}
	}
}
