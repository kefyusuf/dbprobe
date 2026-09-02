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

type precedenceAdapter struct{}

func (precedenceAdapter) Metadata() adapter.Metadata {
	return adapter.Metadata{ID: "precedence-test", Name: "Precedence Test", Version: "1", ContractVersion: adapter.ContractVersion}
}
func (precedenceAdapter) Match(spec adapter.TargetSpec) bool { return spec.Scheme == "precedence" }
func (precedenceAdapter) Open(context.Context, adapter.TargetSpec, adapter.OpenOptions) (adapter.Runtime, error) {
	return precedenceRuntime{}, nil
}

type precedenceRuntime struct{}

func (precedenceRuntime) Target() adapter.TargetMetadata {
	return adapter.TargetMetadata{Engine: "test", AdapterID: "precedence-test", Fingerprint: "one", DisplayName: "one"}
}
func (precedenceRuntime) Capabilities() capability.Set { return capability.New() }
func (precedenceRuntime) Collectors() []collector.Collector {
	return []collector.Collector{precedenceCollector{}}
}
func (precedenceRuntime) Rules() []finding.Rule { return []finding.Rule{duplicateCoreRule{}} }
func (precedenceRuntime) SecurityProfile() adapter.SecurityProfile {
	return adapter.SecurityProfile{ReadOnlyGuaranteed: true}
}
func (precedenceRuntime) Close() error { return nil }

type precedenceCollector struct{}

func (precedenceCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{ID: "precedence.connections", Strategy: collector.StrategySnapshot}
}
func (precedenceCollector) Collect(_ context.Context, request collector.Request) ([]signal.Observation, error) {
	ref := object.Ref{Kind: "test.instance", ID: "one"}
	return []signal.Observation{
		signal.NumberObservation("core.connections.used", ref, 90, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, request.CollectedAt),
		signal.NumberObservation("core.connections.limit", ref, 100, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, request.CollectedAt),
	}, nil
}

type duplicateCoreRule struct{}

func (duplicateCoreRule) ID() finding.ID                    { return "core.connection_saturation" }
func (duplicateCoreRule) Requires() []capability.Capability { return nil }
func (duplicateCoreRule) Evaluate(finding.AnalysisContext) []finding.Finding {
	panic("adapter duplicate core rule must not be evaluated")
}

type precedenceWaiter struct{}

func (precedenceWaiter) Wait(context.Context, time.Duration) error { return nil }

func TestCoreRuleIDsTakePrecedenceOverAdapterDuplicates(t *testing.T) {
	registry, err := adapterregistry.New(precedenceAdapter{})
	if err != nil {
		t.Fatal(err)
	}
	report, err := inspect.New(registry, collection.New(precedenceWaiter{}, time.Now)).Run(context.Background(), "precedence://local", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 || report.Findings[0].ID != "core.connection_saturation" {
		t.Fatalf("findings=%#v", report.Findings)
	}
}
