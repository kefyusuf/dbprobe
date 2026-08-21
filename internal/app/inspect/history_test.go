package inspect

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/collection"
	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/internal/platform/baseline"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestPersistHistoryStoresVersionedSnapshot(t *testing.T) {
	store := baseline.NewMemory()
	at := time.Date(2026, 8, 21, 8, 30, 0, 0, time.UTC)
	report := Report{
		SchemaVersion: SchemaVersion,
		CollectedAt:   at,
		Target: adapter.TargetMetadata{
			Engine:      "mysql",
			AdapterID:   "mysql",
			Fingerprint: "target-fingerprint",
			DisplayName: "db.example:3306/shop",
		},
		Observations: []signal.Observation{
			signal.NumberObservation("core.connections.used", object.Ref{Kind: "mysql.instance", ID: "server"}, 12, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, at),
		},
		Deltas:   []signal.Delta{},
		Findings: nil,
		Warnings: []collection.Warning{},
	}
	if warning := persistHistory(context.Background(), store, report, "0.1.0"); warning != nil {
		t.Fatalf("unexpected warning: %#v", warning)
	}
	latest, err := store.Latest(context.Background(), "target-fingerprint")
	if err != nil {
		t.Fatal(err)
	}
	if latest.SchemaVersion != temporal.SnapshotSchemaVersion || latest.AdapterVersion != "0.1.0" || latest.TargetFingerprint != "target-fingerprint" {
		t.Fatalf("snapshot=%#v", latest)
	}
	if len(latest.Observations) != 1 || latest.Observations[0].Key != "core.connections.used" {
		t.Fatalf("observations=%#v", latest.Observations)
	}
}

type failingHistoryStore struct{}

func (failingHistoryStore) Save(context.Context, temporal.Snapshot) error {
	return errors.New("/secret/path failed")
}
func (failingHistoryStore) Latest(context.Context, string) (*temporal.Snapshot, error) {
	return nil, temporal.ErrSnapshotNotFound
}
func (failingHistoryStore) Previous(context.Context, string, time.Time) (*temporal.Snapshot, error) {
	return nil, temporal.ErrSnapshotNotFound
}
func (failingHistoryStore) List(context.Context, string, int) ([]temporal.Snapshot, error) {
	return nil, temporal.ErrSnapshotNotFound
}

func TestPersistHistoryReturnsSafeWarningOnStoreFailure(t *testing.T) {
	report := Report{
		CollectedAt: time.Now().UTC(),
		Target: adapter.TargetMetadata{
			Engine:      "mysql",
			AdapterID:   "mysql",
			Fingerprint: "target",
		},
	}
	warning := persistHistory(context.Background(), failingHistoryStore{}, report, "0.1.0")
	if warning == nil || warning.CollectorID != "history" || warning.Reason != "snapshot persistence failed" {
		t.Fatalf("warning=%#v", warning)
	}
}
