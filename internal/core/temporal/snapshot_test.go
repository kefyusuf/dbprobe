package temporal

import (
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestNewSnapshotIsDeterministicSortedAndDefensive(t *testing.T) {
	at := time.Date(2026, 8, 21, 7, 0, 0, 123, time.UTC)
	obs := []signal.Observation{signal.NumberObservation("core.connections.used", object.Ref{Kind: "mysql.instance", ID: "abc"}, 12, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, at)}
	in := SnapshotInput{TargetFingerprint: "abc", Engine: "mysql", AdapterID: "mysql", AdapterVersion: "0.1.0", CollectedAt: at, Capabilities: []capability.Capability{"z", "a", "z"}, Observations: obs}
	first, err := NewSnapshot(in)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewSnapshot(in)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || first.ID != second.ID {
		t.Fatalf("unstable id: %q %q", first.ID, second.ID)
	}
	if first.SchemaVersion != SnapshotSchemaVersion {
		t.Fatalf("schema=%q", first.SchemaVersion)
	}
	if len(first.Capabilities) != 2 || first.Capabilities[0] != "a" || first.Capabilities[1] != "z" {
		t.Fatalf("caps=%v", first.Capabilities)
	}
	obs[0].Object.ID = "mutated"
	in.Capabilities[0] = "mutated"
	if first.Observations[0].Object.ID != "abc" || first.Capabilities[0] != "a" {
		t.Fatal("snapshot aliases input")
	}
}

func TestNewSnapshotRejectsIncompleteIdentity(t *testing.T) {
	at := time.Now()
	cases := []SnapshotInput{
		{Engine: "mysql", AdapterID: "mysql", CollectedAt: at},
		{TargetFingerprint: "x", AdapterID: "mysql", CollectedAt: at},
		{TargetFingerprint: "x", Engine: "mysql", CollectedAt: at},
		{TargetFingerprint: "x", Engine: "mysql", AdapterID: "mysql"},
	}
	for _, tc := range cases {
		if _, err := NewSnapshot(tc); err == nil {
			t.Fatalf("expected error for %#v", tc)
		}
	}
}
