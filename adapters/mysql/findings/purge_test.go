package findings

import (
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestPurgeLagThresholdsAreConservative(t *testing.T) {
	ref := object.Ref{Kind: "mysql.instance", ID: "db"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("mysql.innodb_metrics"), Current: []signal.Observation{
		signal.NumberObservation("mysql.innodb.history_list_length", ref, 100000, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, time.Now()),
	}}
	assertPurgeFinding(t, purgeLagRule{}.Evaluate(ctx), "warn")
	ctx.Current[0] = signal.NumberObservation("mysql.innodb.history_list_length", ref, 1000000, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, time.Now())
	assertPurgeFinding(t, purgeLagRule{}.Evaluate(ctx), "critical")
	ctx.Current[0] = signal.NumberObservation("mysql.innodb.history_list_length", ref, 5000, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, time.Now())
	if got := (purgeLagRule{}).Evaluate(ctx); len(got) != 0 {
		t.Fatalf("low history list finding=%#v", got)
	}
}

func assertPurgeFinding(t *testing.T, values []finding.Finding, severity string) {
	t.Helper()
	if len(values) != 1 || values[0].ID != "mysql.innodb_purge_lag" || string(values[0].Severity) != severity {
		t.Fatalf("findings=%#v", values)
	}
}
