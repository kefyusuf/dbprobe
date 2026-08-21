package temporal

import (
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestNewSnapshotIsDeterministicSortedAndDefensive(t *testing.T) {
	at := time.Date(2026, 8, 21, 7, 0, 0, 123, time.UTC)
	observations := []signal.Observation{
		signal.NumberObservation("core.connections.used", object.Ref{Kind: "mysql.instance", ID: "abc"}, 12, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, at),
	}
	input := SnapshotInput{
		TargetFingerprint: "abc",
		Engine:            "mysql",
		AdapterID:         "mysql",
		AdapterVersion:    "0.1.0",
		CollectedAt:       at,
		Capabilities:      []capability.Capability{"z", "a", "z"},
		Observations:      observations,
	}
	first, err := NewSnapshot(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewSnapshot(input)
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
		t.Fatalf("capabilities=%v", first.Capabilities)
	}
	observations[0].Object.ID = "mutated"
	input.Capabilities[0] = "mutated"
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
	for _, testCase := range cases {
		if _, err := NewSnapshot(testCase); err == nil {
			t.Fatalf("expected error for %#v", testCase)
		}
	}
}

func TestNewSnapshotDropsQueryTextFromPersistentEvidence(t *testing.T) {
	at := time.Now().UTC()
	queryText := "SELECT * FROM users WHERE email = 'secret@example.test'"
	queryShape := "SELECT * FROM users WHERE email = ?"
	ref := object.Ref{Kind: "mysql.query", ID: "digest"}
	snapshot, err := NewSnapshot(SnapshotInput{
		TargetFingerprint: "target",
		Engine:            "mysql",
		AdapterID:         "mysql",
		CollectedAt:       at,
		Observations: []signal.Observation{
			{Key: "unsafe.query", Object: ref, Text: &queryText, Exactness: signal.ExactnessScraped, Sensitivity: signal.SensitivityQueryText},
			{Key: "safe.shape", Object: ref, Text: &queryShape, Exactness: signal.ExactnessScraped, Sensitivity: signal.SensitivityQueryShape},
		},
		Findings: []finding.Finding{{
			ID: "test.finding",
			Evidence: []signal.Observation{
				{Key: "unsafe.evidence", Object: ref, Text: &queryText, Exactness: signal.ExactnessScraped, Sensitivity: signal.SensitivityQueryText},
				{Key: "safe.evidence", Object: ref, Text: &queryShape, Exactness: signal.ExactnessScraped, Sensitivity: signal.SensitivityQueryShape},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Observations) != 1 || snapshot.Observations[0].Key != "safe.shape" {
		t.Fatalf("persisted observations=%#v", snapshot.Observations)
	}
	if len(snapshot.Findings) != 1 || len(snapshot.Findings[0].Evidence) != 1 || snapshot.Findings[0].Evidence[0].Key != "safe.evidence" {
		t.Fatalf("persisted finding evidence=%#v", snapshot.Findings)
	}
}
