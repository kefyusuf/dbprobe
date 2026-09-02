package target_test

import (
	"context"
	"testing"

	app "github.com/kefyusuf/dbprobe/internal/app/target"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/finding"
)

type targetTestAdapter struct {
	runtime *targetTestRuntime
	opens   int
}

func (*targetTestAdapter) Metadata() adapter.Metadata {
	return adapter.Metadata{ID: "target-test", Name: "Target Test", Version: "1", ContractVersion: adapter.ContractVersion}
}
func (*targetTestAdapter) Match(spec adapter.TargetSpec) bool { return spec.Scheme == "target-test" }
func (a *targetTestAdapter) Open(context.Context, adapter.TargetSpec, adapter.OpenOptions) (adapter.Runtime, error) {
	a.opens++
	return a.runtime, nil
}

type targetTestRuntime struct {
	target adapter.TargetMetadata
	closed int
}

func (r *targetTestRuntime) Target() adapter.TargetMetadata  { return r.target }
func (*targetTestRuntime) Capabilities() capability.Set      { return capability.New() }
func (*targetTestRuntime) Collectors() []collector.Collector { return nil }
func (*targetTestRuntime) Rules() []finding.Rule             { return nil }
func (*targetTestRuntime) SecurityProfile() adapter.SecurityProfile {
	return adapter.SecurityProfile{ReadOnlyGuaranteed: true}
}
func (r *targetTestRuntime) Close() error { r.closed++; return nil }

func TestResolveReturnsTargetAndClosesRuntime(t *testing.T) {
	runtime := &targetTestRuntime{target: adapter.TargetMetadata{Engine: "mysql", AdapterID: "target-test", Fingerprint: "fp", DisplayName: "shop"}}
	candidate := &targetTestAdapter{runtime: runtime}
	registry, err := adapterregistry.New(candidate)
	if err != nil {
		t.Fatal(err)
	}
	got, err := app.New(registry).Resolve(context.Background(), "target-test://local")
	if err != nil {
		t.Fatal(err)
	}
	if got.Fingerprint != "fp" || got.Engine != "mysql" {
		t.Fatalf("target=%#v", got)
	}
	if runtime.closed != 1 {
		t.Fatalf("runtime close calls=%d", runtime.closed)
	}
}

func TestResolveRejectsInvalidTargetBeforeOpeningAdapter(t *testing.T) {
	candidate := &targetTestAdapter{runtime: &targetTestRuntime{}}
	registry, err := adapterregistry.New(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.New(registry).Resolve(context.Background(), "not-a-url"); err == nil {
		t.Fatal("expected invalid target error")
	}
	if candidate.opens != 0 {
		t.Fatalf("adapter opens=%d", candidate.opens)
	}
}

func TestResolveRejectsIncompleteTargetMetadata(t *testing.T) {
	runtime := &targetTestRuntime{target: adapter.TargetMetadata{Engine: "mysql", AdapterID: "target-test", DisplayName: "shop"}}
	candidate := &targetTestAdapter{runtime: runtime}
	registry, err := adapterregistry.New(candidate)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.New(registry).Resolve(context.Background(), "target-test://local"); err == nil {
		t.Fatal("expected incomplete metadata error")
	}
	if runtime.closed != 1 {
		t.Fatalf("runtime close calls=%d", runtime.closed)
	}
}
