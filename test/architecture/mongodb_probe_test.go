package architecture_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	appdiff "github.com/kefyusuf/dbprobe/internal/app/diff"
	"github.com/kefyusuf/dbprobe/internal/app/inspect"
	"github.com/kefyusuf/dbprobe/internal/core/collection"
	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/internal/platform/baseline"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type mongoProbeWaiter struct{}

func (mongoProbeWaiter) Wait(context.Context, time.Duration) error { return nil }

type mongoProbeAdapter struct {
	mu    sync.Mutex
	opens int
}

func (a *mongoProbeAdapter) Metadata() adapter.Metadata {
	return adapter.Metadata{ID: "mongodb-probe", Name: "MongoDB Probe", Version: "0.1.0", ContractVersion: adapter.ContractVersion}
}
func (a *mongoProbeAdapter) Match(spec adapter.TargetSpec) bool {
	return spec.Scheme == "mongodb-probe"
}
func (a *mongoProbeAdapter) Open(context.Context, adapter.TargetSpec, adapter.OpenOptions) (adapter.Runtime, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.opens++
	return &mongoProbeRuntime{run: a.opens}, nil
}

type mongoProbeRuntime struct {
	run int
}

func (r *mongoProbeRuntime) Target() adapter.TargetMetadata {
	return adapter.TargetMetadata{Engine: "mongodb", AdapterID: "mongodb-probe", Fingerprint: "mongo-cluster-shop", DisplayName: "shop"}
}
func (r *mongoProbeRuntime) Capabilities() capability.Set {
	return capability.New(
		"activity.operations",
		"workload.query_summary",
		"schema.collections",
		"schema.indexes",
		"storage.cache",
		"replication.status",
		"mongodb.wiredtiger",
		"mongodb.replica_set",
		"mongodb.oplog",
	)
}
func (r *mongoProbeRuntime) Collectors() []collector.Collector {
	return []collector.Collector{mongoCacheCollector{run: r.run}, &mongoQueryCollector{run: r.run}}
}
func (r *mongoProbeRuntime) Rules() []finding.Rule { return []finding.Rule{mongoCachePressureRule{}} }
func (r *mongoProbeRuntime) SecurityProfile() adapter.SecurityProfile {
	return adapter.SecurityProfile{ReadOnlyGuaranteed: true}
}
func (r *mongoProbeRuntime) Close() error { return nil }

type mongoCacheCollector struct {
	run int
}

func (c mongoCacheCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mongodb-probe.wiredtiger",
		Requires: []capability.Capability{"mongodb.wiredtiger"},
		Produces: []signal.Key{"mongodb.wiredtiger.cache_pressure_ratio"},
		Strategy: collector.StrategySnapshot,
	}
}
func (c mongoCacheCollector) Collect(_ context.Context, request collector.Request) ([]signal.Observation, error) {
	value := 0.70
	if c.run >= 2 {
		value = 0.95
	}
	return []signal.Observation{
		signal.NumberObservation(
			"mongodb.wiredtiger.cache_pressure_ratio",
			object.Ref{Kind: "mongodb.instance", ID: "cluster"},
			value,
			signal.UnitRatio,
			signal.ExactnessScraped,
			signal.SensitivityMetadata,
			request.CollectedAt,
		),
	}, nil
}

type mongoQueryCollector struct {
	mu    sync.Mutex
	run   int
	calls int
}

func (c *mongoQueryCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "mongodb-probe.query-shape",
		Requires: []capability.Capability{"workload.query_summary"},
		Produces: []signal.Key{"core.query.calls", "mongodb.query.total_latency_ms", "mongodb.query.shape"},
		Strategy: collector.StrategyCounter,
	}
}
func (c *mongoQueryCollector) Collect(_ context.Context, request collector.Request) ([]signal.Observation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++

	calls := 100.0
	latency := 1000.0
	if c.run == 1 && c.calls >= 2 {
		calls = 120
		latency = 1200
	}
	if c.run >= 2 {
		calls = 200
		latency = 2000
		if c.calls >= 2 {
			calls = 230
			latency = 2900
		}
	}

	ref := object.Ref{Kind: "mongodb.query_shape", ID: "shop.orders:shape-hash"}
	shape := "{customer_id: ?, status: ?}"
	return []signal.Observation{
		signal.NumberObservation("core.query.calls", ref, calls, signal.UnitCount, signal.ExactnessCumulative, signal.SensitivityMetadata, request.CollectedAt),
		signal.NumberObservation("mongodb.query.total_latency_ms", ref, latency, signal.UnitMilliseconds, signal.ExactnessCumulative, signal.SensitivityMetadata, request.CollectedAt),
		{Key: "mongodb.query.shape", Object: ref, Text: &shape, Exactness: signal.ExactnessScraped, Sensitivity: signal.SensitivityQueryShape, CollectedAt: request.CollectedAt},
	}, nil
}

type mongoCachePressureRule struct{}

func (mongoCachePressureRule) ID() finding.ID { return "mongodb.wiredtiger_cache_pressure" }
func (mongoCachePressureRule) Requires() []capability.Capability {
	return []capability.Capability{"mongodb.wiredtiger"}
}
func (r mongoCachePressureRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	for _, observation := range ctx.Current {
		if observation.Key == "mongodb.wiredtiger.cache_pressure_ratio" && observation.Number != nil && *observation.Number >= 0.90 {
			return []finding.Finding{{
				ID:         r.ID(),
				Title:      "WiredTiger cache pressure",
				Severity:   "warn",
				Object:     observation.Object,
				Evidence:   []signal.Observation{observation},
				Summary:    fmt.Sprintf("cache pressure %.0f%%", *observation.Number*100),
				Confidence: 0.9,
			}}
		}
	}
	return nil
}

func TestMongoDBSemanticProbeValidatesNonRelationalArchitecture(t *testing.T) {
	probe := &mongoProbeAdapter{}
	registry, err := adapterregistry.New(probe)
	if err != nil {
		t.Fatal(err)
	}
	store := baseline.NewMemory()
	service := inspect.New(registry, collection.New(mongoProbeWaiter{}, time.Now)).WithHistory(store)

	first, err := service.Run(context.Background(), "mongodb-probe://shop", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	second, err := service.Run(context.Background(), "mongodb-probe://shop", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if first.Target.Engine != "mongodb" || second.Target.Engine != "mongodb" {
		t.Fatalf("targets=%#v %#v", first.Target, second.Target)
	}
	if !hasMongoObservation(second, "mongodb.query.shape", "mongodb.query_shape") || !hasMongoObservation(second, "mongodb.wiredtiger.cache_pressure_ratio", "mongodb.instance") {
		t.Fatalf("MongoDB-native observations missing: %#v", second.Observations)
	}
	if len(second.Findings) != 1 || second.Findings[0].ID != "mongodb.wiredtiger_cache_pressure" {
		t.Fatalf("findings=%#v", second.Findings)
	}

	report, err := appdiff.New(store).Run(
		context.Background(),
		second.Target.Fingerprint,
		&temporal.MetricPair{CallsKey: "core.query.calls", TotalLatencyKey: "mongodb.query.total_latency_ms"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.QueryRegressions) != 1 || report.QueryRegressions[0].Object.Kind != "mongodb.query_shape" {
		t.Fatalf("regressions=%#v", report.QueryRegressions)
	}
	foundRegression := false
	for _, event := range report.Events {
		if event.Type == temporal.EventQueryRegression && event.Object.Kind == "mongodb.query_shape" {
			foundRegression = true
		}
	}
	if !foundRegression {
		t.Fatalf("events=%#v", report.Events)
	}
}

func hasMongoObservation(report inspect.Report, key signal.Key, kind string) bool {
	for _, observation := range report.Observations {
		if observation.Key == key && observation.Object.Kind == kind {
			return true
		}
	}
	return false
}
