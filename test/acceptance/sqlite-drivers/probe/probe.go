package probe

import (
	"context"
	"database/sql/driver"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	platformsqlite "github.com/kefyusuf/dbprobe/internal/platform/sqlite"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

const observationsPerSnapshot = 32
const deltasPerSnapshot = 16

type ConnectorFactory func(string) (driver.Connector, error)

type Result struct {
	Driver                  string `json:"driver"`
	GoVersion               string `json:"go_version"`
	GOOS                    string `json:"goos"`
	GOARCH                  string `json:"goarch"`
	Snapshots               int    `json:"snapshots"`
	ObservationsPerSnapshot int    `json:"observations_per_snapshot"`
	DeltasPerSnapshot       int    `json:"deltas_per_snapshot"`
	OpenMigrateNS           int64  `json:"open_migrate_ns"`
	WriteNS                 int64  `json:"write_ns"`
	WritePerSnapshotNS      int64  `json:"write_per_snapshot_ns"`
	ReopenReadNS            int64  `json:"reopen_read_ns"`
	TotalNS                 int64  `json:"total_ns"`
	DatabaseBytes           int64  `json:"database_bytes"`
}

func Run(driverName string, factory ConnectorFactory, snapshotCount int) (Result, error) {
	if driverName == "" {
		return Result{}, fmt.Errorf("driver name is required")
	}
	if factory == nil {
		return Result{}, fmt.Errorf("connector factory is required")
	}
	if snapshotCount < 2 {
		return Result{}, fmt.Errorf("at least two snapshots are required")
	}

	snapshots, err := buildSnapshots(snapshotCount)
	if err != nil {
		return Result{}, err
	}
	root, err := os.MkdirTemp("", "dbprobe-sqlite-driver-")
	if err != nil {
		return Result{}, fmt.Errorf("create comparison directory: %w", err)
	}
	defer os.RemoveAll(root)
	path := filepath.Join(root, "dbprobe.db")
	ctx := context.Background()
	totalStarted := time.Now()

	openStarted := time.Now()
	store, err := platformsqlite.Open(ctx, path, factory)
	if err != nil {
		return Result{}, fmt.Errorf("open %s store: %w", driverName, err)
	}
	openDuration := time.Since(openStarted)

	writeStarted := time.Now()
	for i, snapshot := range snapshots {
		if err := store.Save(ctx, snapshot); err != nil {
			_ = store.Close()
			return Result{}, fmt.Errorf("save %s snapshot %d: %w", driverName, i, err)
		}
	}
	if err := store.Save(ctx, snapshots[0]); err != nil {
		_ = store.Close()
		return Result{}, fmt.Errorf("repeat %s snapshot: %w", driverName, err)
	}
	writeDuration := time.Since(writeStarted)
	if err := store.Close(); err != nil {
		return Result{}, fmt.Errorf("close %s store: %w", driverName, err)
	}

	reopenStarted := time.Now()
	reopened, err := platformsqlite.Open(ctx, path, factory)
	if err != nil {
		return Result{}, fmt.Errorf("reopen %s store: %w", driverName, err)
	}
	latest, err := reopened.Latest(ctx, snapshots[0].TargetFingerprint)
	if err != nil {
		_ = reopened.Close()
		return Result{}, fmt.Errorf("read %s latest snapshot: %w", driverName, err)
	}
	if latest.ID != snapshots[len(snapshots)-1].ID {
		_ = reopened.Close()
		return Result{}, fmt.Errorf("%s latest snapshot mismatch", driverName)
	}
	previous, err := reopened.Previous(ctx, snapshots[0].TargetFingerprint, latest.CollectedAt)
	if err != nil {
		_ = reopened.Close()
		return Result{}, fmt.Errorf("read %s previous snapshot: %w", driverName, err)
	}
	if previous.ID != snapshots[len(snapshots)-2].ID {
		_ = reopened.Close()
		return Result{}, fmt.Errorf("%s previous snapshot mismatch", driverName)
	}
	items, err := reopened.List(ctx, snapshots[0].TargetFingerprint, 0)
	if err != nil {
		_ = reopened.Close()
		return Result{}, fmt.Errorf("list %s snapshots: %w", driverName, err)
	}
	if len(items) != snapshotCount {
		_ = reopened.Close()
		return Result{}, fmt.Errorf("%s snapshot count=%d want=%d", driverName, len(items), snapshotCount)
	}
	conflict := snapshots[0]
	conflict.AdapterVersion = "conflicting-version"
	if err := reopened.Save(ctx, conflict); err == nil {
		_ = reopened.Close()
		return Result{}, fmt.Errorf("%s accepted conflicting snapshot payload", driverName)
	}
	if err := reopened.Close(); err != nil {
		return Result{}, fmt.Errorf("close reopened %s store: %w", driverName, err)
	}
	reopenReadDuration := time.Since(reopenStarted)

	info, err := os.Stat(path)
	if err != nil {
		return Result{}, fmt.Errorf("stat %s database: %w", driverName, err)
	}
	return Result{
		Driver:                  driverName,
		GoVersion:               runtime.Version(),
		GOOS:                    runtime.GOOS,
		GOARCH:                  runtime.GOARCH,
		Snapshots:               snapshotCount,
		ObservationsPerSnapshot: observationsPerSnapshot,
		DeltasPerSnapshot:       deltasPerSnapshot,
		OpenMigrateNS:           openDuration.Nanoseconds(),
		WriteNS:                 writeDuration.Nanoseconds(),
		WritePerSnapshotNS:      writeDuration.Nanoseconds() / int64(snapshotCount),
		ReopenReadNS:            reopenReadDuration.Nanoseconds(),
		TotalNS:                 time.Since(totalStarted).Nanoseconds(),
		DatabaseBytes:           info.Size(),
	}, nil
}

func buildSnapshots(count int) ([]temporal.Snapshot, error) {
	base := time.Unix(1_700_000_000, 0).UTC()
	out := make([]temporal.Snapshot, 0, count)
	for i := 0; i < count; i++ {
		at := base.Add(time.Duration(i) * time.Second)
		observations := make([]signal.Observation, 0, observationsPerSnapshot)
		for j := 0; j < observationsPerSnapshot; j++ {
			observations = append(observations, signal.NumberObservation(
				signal.Key(fmt.Sprintf("core.compare.metric_%02d", j)),
				object.Ref{Kind: "database.metric", ID: fmt.Sprintf("metric-%02d", j)},
				float64(i*observationsPerSnapshot+j),
				signal.UnitCount,
				signal.ExactnessScraped,
				signal.SensitivityMetadata,
				at,
			))
		}
		deltas := make([]signal.Delta, 0, deltasPerSnapshot)
		for j := 0; j < deltasPerSnapshot; j++ {
			deltas = append(deltas, signal.Delta{
				Key:           signal.Key(fmt.Sprintf("core.compare.counter_%02d", j)),
				Object:        object.Ref{Kind: "database.metric", ID: fmt.Sprintf("counter-%02d", j)},
				Unit:          signal.UnitCount,
				Delta:         float64(i + j + 1),
				RatePerSecond: float64(i+j+1) / 10,
				WindowSeconds: 10,
				Exactness:     signal.ExactnessSampled,
			})
		}
		snapshot, err := temporal.NewSnapshot(temporal.SnapshotInput{
			TargetFingerprint: "sqlite-driver-comparison-target",
			Engine:            "comparison",
			AdapterID:         "comparison-adapter",
			AdapterVersion:    "0.1.0",
			CollectedAt:       at,
			Observations:      observations,
			Deltas:            deltas,
		})
		if err != nil {
			return nil, fmt.Errorf("build snapshot %d: %w", i, err)
		}
		out = append(out, snapshot)
	}
	return out, nil
}
