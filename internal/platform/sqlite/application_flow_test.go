package sqlite

import (
	"context"
	"sync"
	"testing"
	"time"

	appdiff "github.com/kefyusuf/dbprobe/internal/app/diff"
	"github.com/kefyusuf/dbprobe/internal/app/inspect"
	"github.com/kefyusuf/dbprobe/internal/core/collection"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type sqliteFlowWaiter struct{}

func (sqliteFlowWaiter) Wait(context.Context, time.Duration) error { return nil }

type sqliteFlowAdapter struct {
	mu    sync.Mutex
	opens int
}

func (*sqliteFlowAdapter) Metadata() adapter.Metadata {
	return adapter.Metadata{ID: "sqlite-flow", Name: "SQLite Flow", Version: "0.1.0", ContractVersion: adapter.ContractVersion}
}
func (*sqliteFlowAdapter) Match(spec adapter.TargetSpec) bool { return spec.Scheme == "sqlite-flow" }
func (a *sqliteFlowAdapter) Open(context.Context, adapter.TargetSpec, adapter.OpenOptions) (adapter.Runtime, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.opens++
	return sqliteFlowRuntime{run: a.opens}, nil
}

type sqliteFlowRuntime struct{ run int }

func (r sqliteFlowRuntime) Target() adapter.TargetMetadata {
	return adapter.TargetMetadata{Engine: "testdb", AdapterID: "sqlite-flow", Fingerprint: "sqlite-flow-target", DisplayName: "flow"}
}
func (r sqliteFlowRuntime) Capabilities() capability.Set { return capability.New() }
func (r sqliteFlowRuntime) Collectors() []collector.Collector {
	return []collector.Collector{sqliteFlowCollector{run: r.run}}
}
func (r sqliteFlowRuntime) Rules() []finding.Rule { return nil }
func (r sqliteFlowRuntime) SecurityProfile() adapter.SecurityProfile {
	return adapter.SecurityProfile{ReadOnlyGuaranteed: true}
}
func (r sqliteFlowRuntime) Close() error { return nil }

type sqliteFlowCollector struct{ run int }

func (c sqliteFlowCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{ID: "sqlite-flow.connections", Strategy: collector.StrategySnapshot}
}
func (c sqliteFlowCollector) Collect(_ context.Context, req collector.Request) ([]signal.Observation, error) {
	used := 80.0
	if c.run >= 2 {
		used = 90
	}
	ref := object.Ref{Kind: "testdb.instance", ID: "one"}
	return []signal.Observation{
		signal.NumberObservation("core.connections.used", ref, used, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt),
		signal.NumberObservation("core.connections.limit", ref, 100, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt),
	}, nil
}

func TestSQLiteStorePersistsInspectHistoryForDiffService(t *testing.T) {
	state := newFakeSQLiteState(0)
	db := openFakeSQLiteDB(t, state)
	defer db.Close()
	store, err := New(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	probe := &sqliteFlowAdapter{}
	registry, err := adapterregistry.New(probe)
	if err != nil {
		t.Fatal(err)
	}
	service := inspect.New(registry, collection.New(sqliteFlowWaiter{}, time.Now)).WithHistory(store)
	first, err := service.Run(context.Background(), "sqlite-flow://local", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	second, err := service.Run(context.Background(), "sqlite-flow://local", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Warnings) != 0 || len(second.Warnings) != 0 {
		t.Fatalf("history warnings: first=%#v second=%#v", first.Warnings, second.Warnings)
	}
	if len(second.Findings) != 1 || second.Findings[0].ID != "core.connection_saturation" {
		t.Fatalf("second findings=%#v", second.Findings)
	}
	report, err := appdiff.New(store).Run(context.Background(), second.Target.Fingerprint, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != appdiff.SchemaVersion || report.TargetFingerprint != second.Target.Fingerprint {
		t.Fatalf("report=%#v", report)
	}
	found := false
	for _, change := range report.Changes {
		if change.Key == "core.connections.used" && change.Object.Kind == "testdb.instance" && change.NumericDelta != nil && *change.NumericDelta == 10 {
			found = true
		}
	}
	if !found {
		t.Fatalf("changes=%#v", report.Changes)
	}
	items, err := store.List(context.Background(), second.Target.Fingerprint, 10)
	if err != nil || len(items) != 2 {
		t.Fatalf("history=%#v err=%v", items, err)
	}
}
