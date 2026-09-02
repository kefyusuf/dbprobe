package findings

import (
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func connectionObservation(key signal.Key, ref object.Ref, value float64) signal.Observation {
	return signal.NumberObservation(key, ref, value, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, time.Unix(1, 0))
}

func TestConnectionSaturationThresholdsAndObjectPairing(t *testing.T) {
	a := object.Ref{Kind: "db.instance", ID: "a"}
	b := object.Ref{Kind: "db.instance", ID: "b"}
	ctx := finding.AnalysisContext{Current: []signal.Observation{
		connectionObservation("core.connections.used", a, 84),
		connectionObservation("core.connections.limit", a, 100),
		connectionObservation("core.connections.used", b, 95),
		connectionObservation("core.connections.limit", b, 100),
	}}

	got := (connectionSaturationRule{}).Evaluate(ctx)
	if len(got) != 1 || got[0].Object != b || got[0].Severity != "critical" {
		t.Fatalf("got=%#v", got)
	}

	ctx.Current[0] = connectionObservation("core.connections.used", a, 85)
	got = (connectionSaturationRule{}).Evaluate(ctx)
	if len(got) != 2 || got[0].Object != a || got[0].Severity != "warn" {
		t.Fatalf("got=%#v", got)
	}
}

func TestConnectionSaturationRequiresCompletePositiveEvidence(t *testing.T) {
	ref := object.Ref{Kind: "db.instance", ID: "a"}
	cases := [][]signal.Observation{
		{connectionObservation("core.connections.used", ref, 99)},
		{connectionObservation("core.connections.used", ref, 99), connectionObservation("core.connections.limit", ref, 0)},
	}
	for _, current := range cases {
		if got := (connectionSaturationRule{}).Evaluate(finding.AnalysisContext{Current: current}); len(got) != 0 {
			t.Fatalf("got=%#v", got)
		}
	}
}

func TestConnectionSaturationHasNoEngineCapabilityRequirement(t *testing.T) {
	if got := (connectionSaturationRule{}).Requires(); len(got) != 0 {
		t.Fatalf("requires=%v", got)
	}
}
