package temporal

import (
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestCompareHandlesChangesAddsRemovesAndCounterReset(t *testing.T) {
	at := time.Now().UTC()
	inst := object.Ref{Kind: "mysql.instance", ID: "x"}
	prev := mustSnapshot(t, at, []signal.Observation{
		signal.NumberObservation("gauge", inst, 10, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, at),
		signal.NumberObservation("counter", inst, 100, signal.UnitCount, signal.ExactnessCumulative, signal.SensitivityMetadata, at),
		signal.NumberObservation("removed", inst, 1, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, at),
	}, nil)
	cur := mustSnapshot(t, at.Add(time.Minute), []signal.Observation{
		signal.NumberObservation("gauge", inst, 13, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, at),
		signal.NumberObservation("counter", inst, 5, signal.UnitCount, signal.ExactnessCumulative, signal.SensitivityMetadata, at),
		signal.NumberObservation("added", inst, 2, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, at),
	}, nil)
	diff, err := Compare(prev, cur)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Changes) != 4 {
		t.Fatalf("changes=%#v", diff.Changes)
	}
	assertChange(t, diff, "added", ChangeAdded, nil)
	assertChange(t, diff, "counter", ChangeReset, nil)
	delta := 3.0
	assertChange(t, diff, "gauge", ChangeChanged, &delta)
	assertChange(t, diff, "removed", ChangeRemoved, nil)
}

func TestCompareRejectsDifferentTargets(t *testing.T) {
	at := time.Now()
	a := mustSnapshotFor(t, "a", at, nil, nil)
	b := mustSnapshotFor(t, "b", at.Add(time.Second), nil, nil)
	if _, err := Compare(a, b); err == nil {
		t.Fatal("expected target mismatch")
	}
}

func assertChange(t *testing.T, diff Diff, key string, kind ChangeKind, delta *float64) {
	t.Helper()
	for _, change := range diff.Changes {
		if string(change.Key) != key {
			continue
		}
		if change.Kind != kind {
			t.Fatalf("%s kind=%s", key, change.Kind)
		}
		if delta == nil && change.NumericDelta != nil {
			t.Fatalf("%s unexpected delta", key)
		}
		if delta != nil && (change.NumericDelta == nil || *change.NumericDelta != *delta) {
			t.Fatalf("%s delta=%v", key, change.NumericDelta)
		}
		return
	}
	t.Fatalf("missing %s", key)
}
