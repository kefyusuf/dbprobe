package explain_test

import (
	"context"
	"strings"
	"testing"

	"github.com/kefyusuf/dbprobe/internal/app/explain"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/finding"
)

type testAdapter struct{ runtime adapter.Runtime }

func (testAdapter) Metadata() adapter.Metadata {
	return adapter.Metadata{ID: "test", Name: "Test", Version: "1", ContractVersion: adapter.ContractVersion}
}
func (testAdapter) Match(spec adapter.TargetSpec) bool { return spec.Scheme == "test" }
func (a testAdapter) Open(context.Context, adapter.TargetSpec, adapter.OpenOptions) (adapter.Runtime, error) {
	return a.runtime, nil
}

type baseRuntime struct{ caps capability.Set }

func (r *baseRuntime) Target() adapter.TargetMetadata {
	return adapter.TargetMetadata{Engine: "testdb", AdapterID: "test", Fingerprint: "fp", DisplayName: "local"}
}
func (r *baseRuntime) Capabilities() capability.Set           { return r.caps }
func (*baseRuntime) Collectors() []collector.Collector       { return nil }
func (*baseRuntime) Rules() []finding.Rule                   { return nil }
func (*baseRuntime) SecurityProfile() adapter.SecurityProfile { return adapter.SecurityProfile{ReadOnlyGuaranteed: true} }
func (*baseRuntime) Close() error                            { return nil }

type explainRuntime struct {
	*baseRuntime
	result    adapter.ExplainResult
	statement string
}

func (r *explainRuntime) ExplainPlan(_ context.Context, req adapter.ExplainRequest) (adapter.ExplainResult, error) {
	r.statement = req.Statement
	return r.result, nil
}

func TestServiceBuildsVersionedReportWithoutEchoingStatement(t *testing.T) {
	runtime := &explainRuntime{
		baseRuntime: &baseRuntime{caps: capability.New("query.explain")},
		result:      adapter.ExplainResult{Engine: "testdb", Format: "test-json", Estimated: true, Plan: `{"plan":1}`},
	}
	registry, _ := adapterregistry.New(testAdapter{runtime: runtime})
	report, err := explain.New(registry).Run(context.Background(), "test://local", "SELECT secret_column FROM users")
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != "dbprobe.explain/v1alpha1" || report.Target.Engine != "testdb" || report.Format != "test-json" || !report.Estimated || report.Plan != `{"plan":1}` {
		t.Fatalf("report=%#v", report)
	}
	if runtime.statement != "SELECT secret_column FROM users" {
		t.Fatalf("statement=%q", runtime.statement)
	}
	if strings.Contains(report.Plan, "secret_column") {
		t.Fatal("test plan unexpectedly contains statement")
	}
}

func TestServiceRequiresCapabilityAndOptionalInterface(t *testing.T) {
	registry, _ := adapterregistry.New(testAdapter{runtime: &baseRuntime{caps: capability.New()}})
	if _, err := explain.New(registry).Run(context.Background(), "test://local", "SELECT 1"); err == nil || !strings.Contains(err.Error(), "capability") {
		t.Fatalf("error=%v", err)
	}

	registry, _ = adapterregistry.New(testAdapter{runtime: &baseRuntime{caps: capability.New("query.explain")}})
	if _, err := explain.New(registry).Run(context.Background(), "test://local", "SELECT 1"); err == nil || !strings.Contains(err.Error(), "contract") {
		t.Fatalf("error=%v", err)
	}
}
