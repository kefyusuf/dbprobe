package inspect_test

import (
	"context"
	"testing"
	"time"

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

type genericNoopWaiter struct{}

func (genericNoopWaiter) Wait(context.Context, time.Duration) error { return nil }

type portableAdapter struct{}

func (portableAdapter) Metadata() adapter.Metadata {
	return adapter.Metadata{ID: "portable-test", Name: "Portable Test", Version: "1", ContractVersion: adapter.ContractVersion}
}
func (portableAdapter) Match(spec adapter.TargetSpec) bool { return spec.Scheme == "portable" }
func (portableAdapter) Open(context.Context, adapter.TargetSpec, adapter.OpenOptions) (adapter.Runtime, error) {
	return portableRuntime{}, nil
}

type portableRuntime struct{}

func (portableRuntime) Target() adapter.TargetMetadata {
	return adapter.TargetMetadata{Engine: "nonmysql", AdapterID: "portable-test", Fingerprint: "portable-1", DisplayName: "portable"}
}
func (portableRuntime) Capabilities() capability.Set { return capability.New() }
func (portableRuntime) Collectors() []collector.Collector {
	return []collector.Collector{portableConnectionCollector{}}
}
func (portableRuntime) Rules() []finding.Rule { return nil }
func (portableRuntime) SecurityProfile() adapter.SecurityProfile {
	return adapter.SecurityProfile{ReadOnlyGuaranteed: true}
}
func (portableRuntime) Close() error { return nil }

type portableConnectionCollector struct{}

func (portableConnectionCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{ID: "portable.connections", Strategy: collector.StrategySnapshot}
}
func (portableConnectionCollector) Collect(_ context.Context, request collector.Request) ([]signal.Observation, error) {
	ref := object.Ref{Kind: "nonmysql.instance", ID: "one"}
	return []signal.Observation{
		signal.NumberObservation("core.connections.used", ref, 90, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, request.CollectedAt),
		signal.NumberObservation("core.connections.limit", ref, 100, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, request.CollectedAt),
	}, nil
}

func TestInspectRunsGenericCoreRulesForNonMySQLAdapter(t *testing.T) {
	registry, err := adapterregistry.New(portableAdapter{})
	if err != nil {
		t.Fatal(err)
	}
	service := inspect.New(registry, collection.New(genericNoopWaiter{}, time.Now))
	report, err := service.Run(context.Background(), "portable://local", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 || report.Findings[0].ID != "core.connection_saturation" || report.Findings[0].Severity != "warn" {
		t.Fatalf("findings=%#v", report.Findings)
	}
	if report.Findings[0].Object.Kind != "nonmysql.instance" || report.Findings[0].Object.ID != "one" {
		t.Fatalf("finding object=%#v", report.Findings[0].Object)
	}
}
