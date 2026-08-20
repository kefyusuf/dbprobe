package fake_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/kefyusuf/dbprobe/adapters/fake"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/collector"
)

func TestFakeAdapterContractValues(t *testing.T) {
	a := fake.New()
	meta := a.Metadata()
	if meta.ID != "fake" { t.Fatalf("ID = %q; want fake", meta.ID) }
	if meta.ContractVersion != adapter.ContractVersion { t.Fatalf("contract = %q; want %q", meta.ContractVersion, adapter.ContractVersion) }
	spec, err := adapter.ParseTarget("fake://local")
	if err != nil { t.Fatal(err) }
	runtime, err := a.Open(context.Background(), spec, adapter.OpenOptions{})
	if err != nil { t.Fatal(err) }
	defer runtime.Close()
	if runtime.Target().Engine != "fake" { t.Fatalf("engine = %q", runtime.Target().Engine) }
	if runtime.Target().Fingerprint == "" { t.Fatal("expected fingerprint") }
	wantCaps := []string{"activity.sessions", "workload.query_summary"}
	gotCaps := make([]string, 0, len(runtime.Capabilities().List()))
	for _, cap := range runtime.Capabilities().List() { gotCaps = append(gotCaps, string(cap)) }
	if !reflect.DeepEqual(gotCaps, wantCaps) { t.Fatalf("capabilities = %#v; want %#v", gotCaps, wantCaps) }
	if !runtime.SecurityProfile().ReadOnlyGuaranteed { t.Fatal("expected read-only guarantee") }
	collectors := runtime.Collectors()
	if len(collectors) != 2 { t.Fatalf("collectors = %d; want 2", len(collectors)) }
	if collectors[0].Descriptor().ID != "fake.health" || collectors[0].Descriptor().Strategy != collector.StrategySnapshot { t.Fatalf("first collector = %#v", collectors[0].Descriptor()) }
	if collectors[1].Descriptor().ID != "fake.workload" || collectors[1].Descriptor().Strategy != collector.StrategyCounter { t.Fatalf("second collector = %#v", collectors[1].Descriptor()) }
}
