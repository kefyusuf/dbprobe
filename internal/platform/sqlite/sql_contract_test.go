package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
)

func TestNewMigratesVersionZeroAndAppliesConnectionPragmas(t *testing.T) {
	state := newFakeSQLiteState(0)
	db := openFakeSQLiteDB(t, state)
	defer db.Close()
	store, err := New(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if store == nil {
		t.Fatal("store is nil")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.userVersion != 1 || state.ddl != 4 || state.begins != 1 || state.commits != 1 {
		t.Fatalf("migration state=%#v", state)
	}
	for _, pragma := range []string{"PRAGMA foreign_keys = ON", "PRAGMA busy_timeout = 5000"} {
		if state.pragmas[pragma] == 0 {
			t.Fatalf("missing pragma %q", pragma)
		}
	}
}

func TestSaveIsTransactionalAndDuplicateSnapshotSkipsTrendRows(t *testing.T) {
	state := newFakeSQLiteState(1)
	db := openFakeSQLiteDB(t, state)
	defer db.Close()
	store, err := New(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := codecSnapshot(t)
	if err := store.Save(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	state.mu.Lock()
	if len(state.snapshots) != 1 || state.trendInserts != 3 || state.commits != 1 {
		state.mu.Unlock()
		t.Fatalf("first save state=%#v", state)
	}
	state.mu.Unlock()
	if err := store.Save(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.snapshots) != 1 || state.trendInserts != 3 || state.commits != 2 {
		t.Fatalf("duplicate save state=%#v", state)
	}
}

func TestSaveRollsBackWhenTrendInsertFails(t *testing.T) {
	state := newFakeSQLiteState(1)
	state.failTrend = true
	db := openFakeSQLiteDB(t, state)
	defer db.Close()
	store, err := New(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), codecSnapshot(t)); err == nil {
		t.Fatal("expected trend insert failure")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.rollbacks != 1 || state.commits != 0 || len(state.snapshots) != 0 || state.trendInserts != 0 {
		t.Fatalf("transaction state=%#v", state)
	}
}

func TestLatestPreviousAndListLoadValidatedPayloads(t *testing.T) {
	state := newFakeSQLiteState(1)
	db := openFakeSQLiteDB(t, state)
	defer db.Close()
	store, err := New(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	first := codecSnapshot(t)
	second, err := temporal.NewSnapshot(temporal.SnapshotInput{
		TargetFingerprint: first.TargetFingerprint,
		Engine:            first.Engine,
		AdapterID:         first.AdapterID,
		AdapterVersion:    first.AdapterVersion,
		CollectedAt:       first.CollectedAt.Add(time.Second),
		Observations:      first.Observations,
		Deltas:            first.Deltas,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	latest, err := store.Latest(context.Background(), first.TargetFingerprint)
	if err != nil || latest.ID != second.ID {
		t.Fatalf("latest=%#v err=%v", latest, err)
	}
	previous, err := store.Previous(context.Background(), first.TargetFingerprint, second.CollectedAt)
	if err != nil || previous.ID != first.ID {
		t.Fatalf("previous=%#v err=%v", previous, err)
	}
	items, err := store.List(context.Background(), first.TargetFingerprint, 2)
	if err != nil || len(items) != 2 || items[0].ID != second.ID || items[1].ID != first.ID {
		t.Fatalf("list=%#v err=%v", items, err)
	}
}
