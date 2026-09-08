package baseline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestMemoryStoreOrdersPartitionsAndReturnsDefensiveCopies(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	at := time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC)
	first := snapshot(t, "target-a", at, 10)
	second := snapshot(t, "target-a", at.Add(time.Minute), 20)
	other := snapshot(t, "target-b", at.Add(2*time.Minute), 99)
	for _, item := range []temporal.Snapshot{second, other, first, second} {
		if err := store.Save(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	latest, err := store.Latest(ctx, "target-a")
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != second.ID {
		t.Fatalf("latest=%q want=%q", latest.ID, second.ID)
	}
	previous, err := store.Previous(ctx, "target-a", second.CollectedAt)
	if err != nil {
		t.Fatal(err)
	}
	if previous.ID != first.ID {
		t.Fatalf("previous=%q want=%q", previous.ID, first.ID)
	}
	listed, err := store.List(ctx, "target-a", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].ID != second.ID || listed[1].ID != first.ID {
		t.Fatalf("list=%#v", listed)
	}
	latest.Observations[0].Object.ID = "mutated"
	again, _ := store.Latest(ctx, "target-a")
	if again.Observations[0].Object.ID == "mutated" {
		t.Fatal("store returned aliased snapshot")
	}
}

func TestMemoryStoreReturnsNotFound(t *testing.T) {
	store := NewMemory()
	if _, err := store.Latest(context.Background(), "missing"); !errors.Is(err, temporal.ErrSnapshotNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func snapshot(t *testing.T, target string, at time.Time, value float64) temporal.Snapshot {
	t.Helper()
	item, err := temporal.NewSnapshot(temporal.SnapshotInput{
		TargetFingerprint: target,
		Engine:            "test",
		AdapterID:         "test",
		AdapterVersion:    "1",
		CollectedAt:       at,
		Observations: []signal.Observation{
			signal.NumberObservation("metric", object.Ref{Kind: "test.object", ID: "one"}, value, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, at),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return item
}
