package diff

import (
	"context"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/internal/platform/baseline"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestServiceBuildsVersionedDiffAndBoundedEventsFromLatestTwoSnapshots(t *testing.T) {
	ctx := context.Background()
	store := baseline.NewMemory()
	at := time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC)
	query := object.Ref{Kind: "mysql.query", ID: "shop:q"}
	previous, _ := temporal.NewSnapshot(temporal.SnapshotInput{
		TargetFingerprint: "target",
		Engine:            "mysql",
		AdapterID:         "mysql",
		AdapterVersion:    "1",
		CollectedAt:       at,
		Observations: []signal.Observation{
			signal.NumberObservation("gauge", object.Ref{Kind: "mysql.instance", ID: "db"}, 10, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, at),
		},
		Deltas: []signal.Delta{
			{Key: "core.query.calls", Object: query, Delta: 20, Exactness: signal.ExactnessSampled},
			{Key: "mysql.query.total_latency_ms", Object: query, Delta: 200, Exactness: signal.ExactnessSampled},
		},
	})
	current, _ := temporal.NewSnapshot(temporal.SnapshotInput{
		TargetFingerprint: "target",
		Engine:            "mysql",
		AdapterID:         "mysql",
		AdapterVersion:    "1",
		CollectedAt:       at.Add(time.Minute),
		Observations: []signal.Observation{
			signal.NumberObservation("gauge", object.Ref{Kind: "mysql.instance", ID: "db"}, 13, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, at),
		},
		Deltas: []signal.Delta{
			{Key: "core.query.calls", Object: query, Delta: 30, Exactness: signal.ExactnessSampled},
			{Key: "mysql.query.total_latency_ms", Object: query, Delta: 900, Exactness: signal.ExactnessSampled},
		},
	})
	_ = store.Save(ctx, previous)
	_ = store.Save(ctx, current)

	service := New(store)
	report, err := service.Run(ctx, "target", &temporal.MetricPair{CallsKey: "core.query.calls", TotalLatencyKey: "mysql.query.total_latency_ms"})
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != SchemaVersion || report.PreviousSnapshotID != previous.ID || report.CurrentSnapshotID != current.ID {
		t.Fatalf("report=%#v", report)
	}
	if len(report.Changes) != 1 || len(report.QueryRegressions) != 1 || len(report.Events) != 1 {
		t.Fatalf("report=%#v", report)
	}
	event := report.Events[0]
	if event.Type != temporal.EventQueryRegression {
		t.Fatalf("event type=%q", event.Type)
	}
	if !event.ObservedAfter.Equal(previous.CollectedAt) || !event.ObservedBefore.Equal(current.CollectedAt) {
		t.Fatalf("event bounds=%v..%v want %v..%v", event.ObservedAfter, event.ObservedBefore, previous.CollectedAt, current.CollectedAt)
	}
	if event.Confidence <= 0 || event.Confidence > 1 {
		t.Fatalf("event confidence=%v", event.Confidence)
	}
}
