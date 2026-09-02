package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestSQLiteHistoryDuplicateIsIdempotentAndConflictIsRejected(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "dbprobe.db")
	snapshot := liveSQLiteSnapshot(t, "target-conflict", time.Unix(300, 0).UTC(), 30)

	store, err := openSQLiteHistory(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.Save(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, snapshot); err != nil {
		t.Fatalf("idempotent duplicate save: %v", err)
	}

	conflicting := snapshot
	conflicting.Observations = append([]signal.Observation(nil), snapshot.Observations...)
	conflicting.Observations[0].Value++
	if err := store.Save(ctx, conflicting); err == nil {
		t.Fatal("expected conflicting same-ID payload to be rejected")
	}

	items, err := store.List(ctx, snapshot.TargetFingerprint, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != snapshot.ID {
		t.Fatalf("items=%#v", items)
	}
}
