package inspect_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/adapters/fake"
	"github.com/kefyusuf/dbprobe/internal/app/inspect"
	"github.com/kefyusuf/dbprobe/internal/core/collection"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type noopWaiter struct{}

func (noopWaiter) Wait(context.Context, time.Duration) error { return nil }

func TestServiceRunsFakeAdapterEndToEnd(t *testing.T) {
	registry, err := adapterregistry.New(fake.New())
	if err != nil {
		t.Fatal(err)
	}
	planner := collection.New(noopWaiter{}, time.Now)
	service := inspect.New(registry, planner)

	report, err := service.Run(context.Background(), "fake://local", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != "dbprobe.inspect/v1alpha1" {
		t.Fatalf("schema_version = %q", report.SchemaVersion)
	}
	if report.Target.Engine != "fake" || report.Target.AdapterID != "fake" {
		t.Fatalf("target = %#v", report.Target)
	}
	wantCaps := []capability.Capability{"activity.sessions", "workload.query_summary"}
	if !reflect.DeepEqual(report.Capabilities, wantCaps) {
		t.Fatalf("capabilities = %#v; want %#v", report.Capabilities, wantCaps)
	}
	wantValues := map[signal.Key]float64{
		"core.connections.used":  12,
		"core.connections.limit": 100,
		"core.query.calls":       140,
	}
	if len(report.Observations) != len(wantValues) {
		t.Fatalf("observations = %#v", report.Observations)
	}
	for _, obs := range report.Observations {
		got, ok := obs.Numeric()
		if !ok || got != wantValues[obs.Key] {
			t.Fatalf("observation %s = %v,%v; want %v", obs.Key, got, ok, wantValues[obs.Key])
		}
	}
	if len(report.Deltas) != 1 || report.Deltas[0].Key != "core.query.calls" || report.Deltas[0].Delta != 40 || report.Deltas[0].RatePerSecond != 4 {
		t.Fatalf("deltas = %#v", report.Deltas)
	}
	if report.Findings == nil || len(report.Findings) != 0 {
		t.Fatalf("findings = %#v; want initialized empty slice", report.Findings)
	}
	if !report.Security.ReadOnlyGuaranteed {
		t.Fatal("expected read-only security profile")
	}
	if report.Warnings == nil || len(report.Warnings) != 0 {
		t.Fatalf("warnings = %#v; want initialized empty slice", report.Warnings)
	}
}
