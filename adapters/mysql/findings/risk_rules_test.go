package findings

import (
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestMissingPrimaryKeyRuleFlagsBaseTableWithoutPrimaryKey(t *testing.T) {
	table := object.Ref{Kind: "mysql.table", ID: "shop.audit_log"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("schema.objects"), Current: []signal.Observation{
		booleanRef("mysql.table.primary_key_present", table, false),
	}}
	assertFinding(t, missingPrimaryKeyRule{}.Evaluate(ctx), "mysql.missing_primary_key", "warn")
}

func TestAutoIncrementExhaustionThresholds(t *testing.T) {
	column := object.Ref{Kind: "mysql.column", ID: "shop.orders.id"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("schema.objects"), Current: []signal.Observation{
		numberRef("mysql.auto_increment.utilization_ratio", column, 0.96),
	}}
	assertFinding(t, autoIncrementExhaustionRule{}.Evaluate(ctx), "mysql.auto_increment_exhaustion", "critical")
	ctx.Current[0] = numberRef("mysql.auto_increment.utilization_ratio", column, 0.79)
	if got := (autoIncrementExhaustionRule{}).Evaluate(ctx); len(got) != 0 {
		t.Fatalf("unexpected finding below threshold: %#v", got)
	}
}

func TestNoGoodIndexAndDiskTempRulesUseWindowDeltas(t *testing.T) {
	query := object.Ref{Kind: "mysql.query", ID: "shop:q"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("workload.query_summary"), Deltas: []signal.Delta{
		delta("core.query.calls", query, 20),
		delta("mysql.query.no_good_index_used", query, 15),
		delta("mysql.query.temp_disk_tables", query, 12),
	}}
	assertFinding(t, noGoodIndexRule{}.Evaluate(ctx), "mysql.no_good_index", "critical")
	assertFinding(t, diskTempTableRule{}.Evaluate(ctx), "mysql.disk_temp_tables", "critical")
}

func TestRedoPressureAndDeadlockRulesRequireSampledActivity(t *testing.T) {
	instance := object.Ref{Kind: "mysql.instance", ID: "db"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("mysql.innodb", "mysql.performance_schema"), Deltas: []signal.Delta{
		delta("mysql.innodb.log_waits", instance, 6),
		delta("mysql.innodb.deadlocks", instance, 11),
	}}
	assertFinding(t, redoPressureRule{}.Evaluate(ctx), "mysql.redo_pressure", "critical")
	assertFinding(t, deadlockRateRule{}.Evaluate(ctx), "mysql.deadlock_rate_high", "critical")
}

func TestFullScanHeavyTableRequiresLargeTableAndWindowReads(t *testing.T) {
	table := object.Ref{Kind: "mysql.table", ID: "shop.orders"}
	ctx := finding.AnalysisContext{
		Capabilities: capability.New("schema.objects", "mysql.performance_schema"),
		Current: []signal.Observation{
			numberRef("mysql.table.estimated_rows", table, 500000),
		},
		Deltas: []signal.Delta{
			delta("mysql.table.full_scan_rows", table, 1200000),
		},
	}
	assertFinding(t, fullScanHeavyTableRule{}.Evaluate(ctx), "mysql.full_scan_heavy_table", "critical")
	ctx.Current[0] = numberRef("mysql.table.estimated_rows", table, 50)
	if got := (fullScanHeavyTableRule{}).Evaluate(ctx); len(got) != 0 {
		t.Fatalf("small table should not trigger: %#v", got)
	}
}
