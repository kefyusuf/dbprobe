package modernc

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	platformsqlite "github.com/kefyusuf/dbprobe/internal/platform/sqlite"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestOpenPersistsSnapshotsAcrossCloseAndReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "dbprobe.db")
	first := liveSnapshot(t, time.Unix(100, 0).UTC(), 10)
	second := liveSnapshot(t, time.Unix(200, 0).UTC(), 20)

	store, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, first); err != nil {
		t.Fatalf("duplicate save must be idempotent: %v", err)
	}
	if err := store.Save(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	latest, err := reopened.Latest(ctx, first.TargetFingerprint)
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != second.ID {
		t.Fatalf("latest=%s want=%s", latest.ID, second.ID)
	}
	previous, err := reopened.Previous(ctx, first.TargetFingerprint, second.CollectedAt)
	if err != nil {
		t.Fatal(err)
	}
	if previous.ID != first.ID {
		t.Fatalf("previous=%s want=%s", previous.ID, first.ID)
	}
	items, err := reopened.List(ctx, first.TargetFingerprint, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != second.ID || items[1].ID != first.ID {
		t.Fatalf("items=%#v", items)
	}

	conflict := first
	conflict.AdapterVersion = "conflicting-version"
	if err := reopened.Save(ctx, conflict); err == nil || !strings.Contains(err.Error(), "identity collision") {
		t.Fatalf("conflicting save error=%v", err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("database mode=%#o want=0600", got)
		}
	}
}

func TestConnectorHonorsPragmasTransactionsAndForeignKeys(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "dbprobe.db")
	connector, err := newConnector(path)
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer db.Close()

	store, err := platformsqlite.New(ctx, db)
	if err != nil {
		t.Fatal(err)
	}

	if got := pragmaInt(t, db, "foreign_keys"); got != 1 {
		t.Fatalf("foreign_keys=%d want=1", got)
	}
	if got := pragmaInt(t, db, "busy_timeout"); got != 5000 {
		t.Fatalf("busy_timeout=%d want=5000", got)
	}
	if got := pragmaInt(t, db, "user_version"); got != 1 {
		t.Fatalf("user_version=%d want=1", got)
	}

	if _, err := db.ExecContext(ctx, `CREATE TRIGGER fail_trend
BEFORE INSERT ON trend_metrics
BEGIN
    SELECT RAISE(ABORT, 'forced trend failure');
END`); err != nil {
		t.Fatal(err)
	}

	snapshot := liveSnapshot(t, time.Unix(300, 0).UTC(), 30)
	if err := store.Save(ctx, snapshot); err == nil || !strings.Contains(err.Error(), "insert SQLite trend metric") {
		t.Fatalf("save error=%v", err)
	}
	if got := rowCount(t, db, "SELECT COUNT(*) FROM snapshots WHERE id = ?", snapshot.ID); got != 0 {
		t.Fatalf("snapshot rows after rollback=%d want=0", got)
	}

	if _, err := db.ExecContext(ctx, "DROP TRIGGER fail_trend"); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	if got := rowCount(t, db, "SELECT COUNT(*) FROM trend_metrics WHERE snapshot_id = ?", snapshot.ID); got == 0 {
		t.Fatal("expected derived trend rows")
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM snapshots WHERE id = ?", snapshot.ID); err != nil {
		t.Fatal(err)
	}
	if got := rowCount(t, db, "SELECT COUNT(*) FROM trend_metrics WHERE snapshot_id = ?", snapshot.ID); got != 0 {
		t.Fatalf("trend rows after cascade=%d want=0", got)
	}
}

func liveSnapshot(t *testing.T, at time.Time, value float64) temporal.Snapshot {
	t.Helper()
	snapshot, err := temporal.NewSnapshot(temporal.SnapshotInput{
		TargetFingerprint: "target-live-modernc",
		Engine:            "testdb",
		AdapterID:         "test-adapter",
		AdapterVersion:    "0.1.0",
		CollectedAt:       at,
		Observations: []signal.Observation{
			signal.NumberObservation(
				"core.connections.current",
				object.Ref{Kind: "database.instance", ID: "primary"},
				value,
				signal.UnitCount,
				signal.ExactnessScraped,
				signal.SensitivityMetadata,
				at,
			),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func pragmaInt(t *testing.T, db *sql.DB, name string) int {
	t.Helper()
	var value int
	if err := db.QueryRow("PRAGMA " + name).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func rowCount(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var count int
	if err := db.QueryRow(query, args...).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
