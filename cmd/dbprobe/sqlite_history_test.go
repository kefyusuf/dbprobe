package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestSQLiteHistoryPersistsAcrossCloseAndReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "dbprobe.db")
	target := "target-live-sqlite"

	first := liveSQLiteSnapshot(t, target, time.Unix(100, 0).UTC(), 10)
	second := liveSQLiteSnapshot(t, target, time.Unix(200, 0).UTC(), 20)

	store, err := openSQLiteHistory(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := openSQLiteHistory(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	latest, err := reopened.Latest(ctx, target)
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != second.ID {
		t.Fatalf("latest=%q want=%q", latest.ID, second.ID)
	}

	previous, err := reopened.Previous(ctx, target, latest.CollectedAt)
	if err != nil {
		t.Fatal(err)
	}
	if previous.ID != first.ID {
		t.Fatalf("previous=%q want=%q", previous.ID, first.ID)
	}

	items, err := reopened.List(ctx, target, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != second.ID || items[1].ID != first.ID {
		t.Fatalf("items=%#v", items)
	}
}

func TestProductionHistoryDependenciesPersistInspectAndEnableDiff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dbprobe.db")
	deps := productionCommandDependencies()
	deps.historyPath = func() (string, error) { return path, nil }

	for i := 0; i < 2; i++ {
		stdout := executeRoot(t, deps, "inspect", "fake://local", "--format=json", "--sample-window=1ms")
		assertSchemaVersion(t, stdout, "dbprobe.inspect/v1alpha1")
	}

	stdout := executeRoot(t, deps, "diff", "fake://local", "--format=json")
	assertSchemaVersion(t, stdout, "dbprobe.diff/v1alpha1")
}

func TestDefaultRootRegistersPersistentDiff(t *testing.T) {
	cmd := newRootCommand()
	found, _, err := cmd.Find([]string{"diff"})
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.Name() != "diff" {
		t.Fatalf("diff command=%#v", found)
	}
}

func liveSQLiteSnapshot(t *testing.T, target string, collectedAt time.Time, value float64) temporal.Snapshot {
	t.Helper()
	snapshot, err := temporal.NewSnapshot(temporal.SnapshotInput{
		TargetFingerprint: target,
		Engine:            "fake",
		AdapterID:         "fake",
		AdapterVersion:    "0.1.0",
		CollectedAt:       collectedAt,
		Capabilities:      []capability.Capability{{ID: "activity.sessions"}},
		Observations: []signal.Observation{{
			Signal: signal.Signal{Key: "core.connections.current", Unit: "count", Type: signal.Gauge},
			Value:  value,
			Object: adapter.ObjectRef{Kind: "database.instance", Name: "local"},
		}},
		Deltas:   []signal.Delta{},
		Findings: []finding.Finding{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
