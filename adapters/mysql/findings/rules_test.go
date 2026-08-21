package findings

import (
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestConnectionSaturationThresholds(t *testing.T) {
	ctx := finding.AnalysisContext{Capabilities: capability.New("mysql.performance_schema"), Current: []signal.Observation{
		number("core.connections.used", "mysql.instance", "db", 96),
		number("core.connections.limit", "mysql.instance", "db", 100),
	}}
	got := connectionSaturationRule{}.Evaluate(ctx)
	assertFinding(t, got, "core.connection_saturation", "critical")
}

func TestLongTransactionDoesNotUseEphemeralObjectAsFindingIdentity(t *testing.T) {
	ctx := finding.AnalysisContext{Capabilities: capability.New("activity.transactions"), Current: []signal.Observation{
		number("mysql.transaction.age_seconds", "mysql.transaction", "ephemeral:123", 1900),
	}}
	got := longTransactionRule{}.Evaluate(ctx)
	assertFinding(t, got, "mysql.long_transaction", "critical")
	if got[0].Object.ID == "ephemeral:123" {
		t.Fatal("finding uses ephemeral transaction id as suppression identity")
	}
}

func TestQueryRulesRequireWindowDeltas(t *testing.T) {
	query := object.Ref{Kind: "mysql.query", ID: "shop:ABC"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("workload.query_summary"), Deltas: []signal.Delta{
		delta("core.query.calls", query, 20),
		delta("mysql.query.no_index_used", query, 18),
		delta("mysql.query.rows_examined", query, 500000),
		delta("mysql.query.rows_sent", query, 100),
	}}
	got := append(queryFullScanRule{}.Evaluate(ctx), queryAmplificationRule{}.Evaluate(ctx)...)
	assertFinding(t, got, "mysql.query_full_scan_heavy", "warn")
	assertFinding(t, got, "mysql.query_rows_examined_amplification", "critical")

	withoutDeltas := finding.AnalysisContext{Capabilities: capability.New("workload.query_summary")}
	if len(queryFullScanRule{}.Evaluate(withoutDeltas)) != 0 || len(queryAmplificationRule{}.Evaluate(withoutDeltas)) != 0 {
		t.Fatal("query rules fired without bounded-window deltas")
	}
}

func TestUnusedIndexRequiresObservationAgeAndNeverTargetsUniqueIndex(t *testing.T) {
	idx := object.Ref{Kind: "mysql.index", ID: "shop.orders.idx_customer"}
	base := []signal.Observation{
		numberRef("mysql.index.reads", idx, 0),
		booleanRef("mysql.index.primary", idx, false),
		booleanRef("mysql.index.unique", idx, false),
		number("mysql.server.uptime_seconds", "mysql.instance", "db", 8*24*3600),
	}
	ctx := finding.AnalysisContext{Capabilities: capability.New("schema.indexes", "mysql.performance_schema"), Current: base}
	got := unusedIndexRule{}.Evaluate(ctx)
	assertFinding(t, got, "mysql.unused_index", "info")
	if strings.Contains(strings.ToLower(got[0].Guidance), "drop this index") {
		t.Fatal("unused index finding gives unconditional DROP guidance")
	}

	short := append([]signal.Observation(nil), base...)
	short[len(short)-1] = number("mysql.server.uptime_seconds", "mysql.instance", "db", 1800)
	ctx.Current = short
	if len(unusedIndexRule{}.Evaluate(ctx)) != 0 {
		t.Fatal("unused index fired on <1h observation window")
	}

	unique := append([]signal.Observation(nil), base...)
	unique[2] = booleanRef("mysql.index.unique", idx, true)
	ctx.Current = unique
	if len(unusedIndexRule{}.Evaluate(ctx)) != 0 {
		t.Fatal("unused-index rule flagged a unique index")
	}
}

func TestRedundantIndexUsesExplicitTableAndColumnMetadata(t *testing.T) {
	short := object.Ref{Kind: "mysql.index", ID: "shop.orders.idx_customer"}
	long := object.Ref{Kind: "mysql.index", ID: "shop.orders.idx_customer_created"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("schema.indexes"), Current: []signal.Observation{
		textRef("mysql.index.table", short, "shop.orders"), textRef("mysql.index.columns", short, "customer_id"), booleanRef("mysql.index.unique", short, false), booleanRef("mysql.index.primary", short, false),
		textRef("mysql.index.table", long, "shop.orders"), textRef("mysql.index.columns", long, "customer_id,created_at"), booleanRef("mysql.index.unique", long, false), booleanRef("mysql.index.primary", long, false),
	}}
	got := redundantIndexRule{}.Evaluate(ctx)
	assertFinding(t, got, "mysql.redundant_index", "info")
	if got[0].Object.ID != short.ID {
		t.Fatalf("redundant object = %q", got[0].Object.ID)
	}
}

func TestBufferPoolRuleUsesWindowTrafficFloor(t *testing.T) {
	instance := object.Ref{Kind: "mysql.instance", ID: "db"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("storage.cache"), Deltas: []signal.Delta{
		delta("mysql.innodb.buffer_pool.read_requests", instance, 20000),
		delta("mysql.innodb.buffer_pool.reads", instance, 4000),
	}}
	got := bufferPoolHitRule{}.Evaluate(ctx)
	assertFinding(t, got, "mysql.buffer_pool_hit_low", "warn")

	ctx.Deltas[0].Delta = 100
	if len(bufferPoolHitRule{}.Evaluate(ctx)) != 0 {
		t.Fatal("buffer pool finding fired below traffic floor")
	}
}

func TestReplicationRulesUseStateAndNumericErrorOnly(t *testing.T) {
	channel := object.Ref{Kind: "mysql.replication_channel", ID: "default"}
	worker := object.Ref{Kind: "mysql.replication_worker", ID: "default:worker:1"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("replication.status"), Current: []signal.Observation{
		textRef("mysql.replication.receiver_state", channel, "CONNECTING"),
		textRef("mysql.replication.applier_state", channel, "OFF"),
		numberRef("mysql.replication.error_code", worker, 1062),
	}}
	stopped := replicationStoppedRule{}.Evaluate(ctx)
	if len(stopped) != 1 || stopped[0].Object.ID != "default" {
		t.Fatalf("stopped findings = %#v", stopped)
	}
	errors := replicationErrorRule{}.Evaluate(ctx)
	assertFinding(t, errors, "mysql.replication_error", "critical")
}

func TestLockContentionAggregatesStableObjects(t *testing.T) {
	table := object.Ref{Kind: "mysql.table", ID: "shop.orders"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("locking.wait_graph"), Current: []signal.Observation{
		numberRef("mysql.lock_wait.count", table, 1),
		numberRef("mysql.lock_wait.count", table, 1),
		numberRef("mysql.lock_wait.count", table, 1),
	}}
	got := lockContentionRule{}.Evaluate(ctx)
	assertFinding(t, got, "mysql.lock_wait_contention", "warn")
}

func assertFinding(t *testing.T, findings []finding.Finding, id, severity string) {
	t.Helper()
	for _, item := range findings {
		if string(item.ID) == id {
			if string(item.Severity) != severity {
				t.Fatalf("%s severity = %q, want %q", id, item.Severity, severity)
			}
			return
		}
	}
	t.Fatalf("missing finding %s in %#v", id, findings)
}

func number(key signal.Key, kind, id string, value float64) signal.Observation {
	return numberRef(key, object.Ref{Kind: kind, ID: id}, value)
}
func numberRef(key signal.Key, ref object.Ref, value float64) signal.Observation {
	return signal.NumberObservation(key, ref, value, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, time.Unix(1, 0))
}
func booleanRef(key signal.Key, ref object.Ref, value bool) signal.Observation {
	return signal.Observation{Key: key, Object: ref, Boolean: &value, Exactness: signal.ExactnessScraped}
}
func textRef(key signal.Key, ref object.Ref, value string) signal.Observation {
	return signal.Observation{Key: key, Object: ref, Text: &value, Exactness: signal.ExactnessScraped}
}
func delta(key signal.Key, ref object.Ref, value float64) signal.Delta {
	return signal.Delta{Key: key, Object: ref, Delta: value, Exactness: signal.ExactnessSampled, WindowSeconds: 10}
}
