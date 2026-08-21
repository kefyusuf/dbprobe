package findings

import (
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestReplicationApplyLagThresholds(t *testing.T) {
	ref := object.Ref{Kind: "mysql.replication_channel", ID: "default"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("replication.status"), Current: []signal.Observation{
		signal.NumberObservation("mysql.replication.active_apply_lag_seconds", ref, 45, signal.UnitSeconds, signal.ExactnessScraped, signal.SensitivityMetadata, time.Now()),
	}}
	got := replicationApplyLagRule{}.Evaluate(ctx)
	assertReplicationLagFinding(t, got, "warn")

	ctx.Current[0] = signal.NumberObservation("mysql.replication.active_apply_lag_seconds", ref, 180, signal.UnitSeconds, signal.ExactnessScraped, signal.SensitivityMetadata, time.Now())
	got = replicationApplyLagRule{}.Evaluate(ctx)
	assertReplicationLagFinding(t, got, "critical")

	ctx.Current[0] = signal.NumberObservation("mysql.replication.active_apply_lag_seconds", ref, 20, signal.UnitSeconds, signal.ExactnessScraped, signal.SensitivityMetadata, time.Now())
	if got := (replicationApplyLagRule{}).Evaluate(ctx); len(got) != 0 {
		t.Fatalf("finding below threshold: %#v", got)
	}
}

func assertReplicationLagFinding(t *testing.T, values []finding.Finding, severity string) {
	t.Helper()
	if len(values) != 1 || values[0].ID != "mysql.replication_apply_lag" || string(values[0].Severity) != severity || values[0].Object.ID != "default" {
		t.Fatalf("findings=%#v", values)
	}
}
