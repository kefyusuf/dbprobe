package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/finding"
)

type cliExplainAdapter struct{ runtime *cliExplainRuntime }

func (cliExplainAdapter) Metadata() adapter.Metadata {
	return adapter.Metadata{ID: "cli-explain", Name: "CLI Explain", Version: "1", ContractVersion: adapter.ContractVersion}
}
func (cliExplainAdapter) Match(spec adapter.TargetSpec) bool { return spec.Scheme == "testplan" }
func (a cliExplainAdapter) Open(context.Context, adapter.TargetSpec, adapter.OpenOptions) (adapter.Runtime, error) {
	return a.runtime, nil
}

type cliExplainRuntime struct{ statement string }

func (*cliExplainRuntime) Target() adapter.TargetMetadata {
	return adapter.TargetMetadata{Engine: "testdb", AdapterID: "cli-explain", Fingerprint: "fp", DisplayName: "local"}
}
func (*cliExplainRuntime) Capabilities() capability.Set     { return capability.New("query.explain") }
func (*cliExplainRuntime) Collectors() []collector.Collector { return nil }
func (*cliExplainRuntime) Rules() []finding.Rule           { return nil }
func (*cliExplainRuntime) SecurityProfile() adapter.SecurityProfile {
	return adapter.SecurityProfile{ReadOnlyGuaranteed: true}
}
func (*cliExplainRuntime) Close() error { return nil }
func (r *cliExplainRuntime) ExplainPlan(_ context.Context, request adapter.ExplainRequest) (adapter.ExplainResult, error) {
	r.statement = request.Statement
	return adapter.ExplainResult{
		Engine:    "testdb",
		Format:    "test-json-sanitized",
		Estimated: true,
		Sanitized: true,
		Plan:      `{"query_block":{"select_id":1}}`,
	}, nil
}

func TestExplainCommandRendersInjectedSanitizedJSON(t *testing.T) {
	runtime := &cliExplainRuntime{}
	factory := func() (*adapterregistry.Registry, error) {
		return adapterregistry.New(cliExplainAdapter{runtime: runtime})
	}
	cmd := newExplainCommandWithRegistry(factory)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"testplan://local", "--statement", "SELECT 1", "--format=json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error=%v stderr=%q", err, stderr.String())
	}
	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if report["schema_version"] != "dbprobe.explain/v1alpha1" || report["format"] != "test-json-sanitized" || report["estimated"] != true || report["sanitized"] != true {
		t.Fatalf("report=%#v", report)
	}
	if runtime.statement != "SELECT 1" {
		t.Fatalf("statement=%q", runtime.statement)
	}
	if strings.Contains(stdout.String(), "SELECT 1") {
		t.Fatal("explain report echoed input statement")
	}
}

func TestExplainCommandValidatesBeforeBuildingRegistry(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "format", args: []string{"testplan://local", "--statement", "SELECT 1", "--format=xml"}, want: "format"},
		{name: "statement", args: []string{"testplan://local", "--statement", "   "}, want: "statement"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			cmd := newExplainCommandWithRegistry(func() (*adapterregistry.Registry, error) {
				calls++
				return nil, nil
			})
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v", err)
			}
			if calls != 0 {
				t.Fatalf("registry factory calls=%d; want 0", calls)
			}
		})
	}
}
