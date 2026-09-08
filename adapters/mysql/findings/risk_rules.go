package findings

import (
	"fmt"
	"sort"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func RiskRules() []finding.Rule {
	return []finding.Rule{
		missingPrimaryKeyRule{},
		autoIncrementExhaustionRule{},
		noGoodIndexRule{},
		diskTempTableRule{},
		redoPressureRule{},
		deadlockRateRule{},
		fullScanHeavyTableRule{},
	}
}

type missingPrimaryKeyRule struct{}

func (missingPrimaryKeyRule) ID() finding.ID { return "mysql.missing_primary_key" }
func (missingPrimaryKeyRule) Requires() []capability.Capability {
	return []capability.Capability{"schema.objects"}
}
func (r missingPrimaryKeyRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	out := []finding.Finding{}
	for _, observation := range ctx.Current {
		if observation.Key != "mysql.table.primary_key_present" || observation.Boolean == nil || *observation.Boolean {
			continue
		}
		out = append(out, finding.Finding{
			ID:         r.ID(),
			Title:      "Table has no explicit primary key",
			Severity:   "warn",
			Object:     observation.Object,
			Evidence:   []signal.Observation{observation},
			Summary:    "The table has no explicit PRIMARY KEY in schema metadata.",
			Guidance:   "Review access patterns, uniqueness requirements, replication/online-schema-change constraints, and table size before adding a primary key.",
			Confidence: 0.98,
		})
	}
	return out
}

type autoIncrementExhaustionRule struct{}

func (autoIncrementExhaustionRule) ID() finding.ID { return "mysql.auto_increment_exhaustion" }
func (autoIncrementExhaustionRule) Requires() []capability.Capability {
	return []capability.Capability{"schema.objects"}
}
func (r autoIncrementExhaustionRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	out := []finding.Finding{}
	for _, observation := range ctx.Current {
		if observation.Key != "mysql.auto_increment.utilization_ratio" || observation.Number == nil {
			continue
		}
		ratio := *observation.Number
		var severity finding.Severity
		switch {
		case ratio >= 0.95:
			severity = "critical"
		case ratio >= 0.80:
			severity = "warn"
		default:
			continue
		}
		out = append(out, finding.Finding{
			ID:         r.ID(),
			Title:      "AUTO_INCREMENT range is nearing exhaustion",
			Severity:   severity,
			Object:     observation.Object,
			Evidence:   []signal.Observation{observation},
			Summary:    fmt.Sprintf("AUTO_INCREMENT utilization is approximately %.1f%% of the column type range.", ratio*100),
			Guidance:   "Confirm the column type/signedness and growth rate, then plan a schema migration before exhaustion. dbprobe does not alter the column automatically.",
			Confidence: 0.95,
		})
	}
	return out
}

type noGoodIndexRule struct{}

func (noGoodIndexRule) ID() finding.ID { return "mysql.no_good_index" }
func (noGoodIndexRule) Requires() []capability.Capability {
	return []capability.Capability{"workload.query_summary"}
}
func (r noGoodIndexRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	out := []finding.Finding{}
	for _, group := range sortedDeltaGroups(ctx.Deltas) {
		calls, okCalls := group.values["core.query.calls"]
		noGood, okNoGood := group.values["mysql.query.no_good_index_used"]
		if !okCalls || !okNoGood || calls < 10 || noGood < 5 {
			continue
		}
		ratio := noGood / calls
		var severity finding.Severity
		switch {
		case ratio >= 0.75:
			severity = "critical"
		case ratio >= 0.25:
			severity = "warn"
		default:
			continue
		}
		out = append(out, finding.Finding{
			ID:         r.ID(),
			Title:      "Query frequently reports no good index",
			Severity:   severity,
			Object:     group.ref,
			Summary:    fmt.Sprintf("%.0f of %.0f sampled executions reported no good index usage (%.1f%%).", noGood, calls, ratio*100),
			Guidance:   "Review the normalized query shape, selectivity, statistics, and a plan-only EXPLAIN before changing indexes.",
			Confidence: 0.88,
		})
	}
	return out
}

type diskTempTableRule struct{}

func (diskTempTableRule) ID() finding.ID { return "mysql.disk_temp_tables" }
func (diskTempTableRule) Requires() []capability.Capability {
	return []capability.Capability{"workload.query_summary"}
}
func (r diskTempTableRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	out := []finding.Finding{}
	for _, group := range sortedDeltaGroups(ctx.Deltas) {
		calls, okCalls := group.values["core.query.calls"]
		diskTables, okDisk := group.values["mysql.query.temp_disk_tables"]
		if !okCalls || !okDisk || calls < 10 || diskTables < 3 {
			continue
		}
		ratio := diskTables / calls
		var severity finding.Severity
		switch {
		case ratio >= 0.50:
			severity = "critical"
		case ratio >= 0.10:
			severity = "warn"
		default:
			continue
		}
		out = append(out, finding.Finding{
			ID:         r.ID(),
			Title:      "Query frequently creates on-disk temporary tables",
			Severity:   severity,
			Object:     group.ref,
			Summary:    fmt.Sprintf("%.0f on-disk temporary tables were created across %.0f sampled executions (%.1f%%).", diskTables, calls, ratio*100),
			Guidance:   "Inspect GROUP BY/ORDER BY/DISTINCT behavior, row width, result cardinality, and relevant memory limits before tuning server settings.",
			Confidence: 0.88,
		})
	}
	return out
}

type redoPressureRule struct{}

func (redoPressureRule) ID() finding.ID { return "mysql.redo_pressure" }
func (redoPressureRule) Requires() []capability.Capability {
	return []capability.Capability{"mysql.innodb", "mysql.performance_schema"}
}
func (r redoPressureRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	out := []finding.Finding{}
	for _, group := range sortedDeltaGroups(ctx.Deltas) {
		waits, ok := group.values["mysql.innodb.log_waits"]
		if !ok || waits < 1 {
			continue
		}
		severity := finding.Severity("warn")
		if waits >= 5 {
			severity = "critical"
		}
		out = append(out, finding.Finding{
			ID:         r.ID(),
			Title:      "InnoDB redo/log waits occurred",
			Severity:   severity,
			Object:     group.ref,
			Summary:    fmt.Sprintf("%.0f InnoDB log waits occurred in the sampled window.", waits),
			Guidance:   "Correlate log waits with write throughput, transaction size, log buffer/redo capacity, fsync latency, and checkpoint pressure before changing configuration.",
			Confidence: 0.90,
		})
	}
	return out
}

type deadlockRateRule struct{}

func (deadlockRateRule) ID() finding.ID { return "mysql.deadlock_rate_high" }
func (deadlockRateRule) Requires() []capability.Capability {
	return []capability.Capability{"mysql.innodb", "mysql.performance_schema"}
}
func (r deadlockRateRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	out := []finding.Finding{}
	for _, group := range sortedDeltaGroups(ctx.Deltas) {
		deadlocks, ok := group.values["mysql.innodb.deadlocks"]
		if !ok || deadlocks < 3 {
			continue
		}
		severity := finding.Severity("warn")
		if deadlocks >= 10 {
			severity = "critical"
		}
		out = append(out, finding.Finding{
			ID:         r.ID(),
			Title:      "InnoDB deadlock activity is elevated",
			Severity:   severity,
			Object:     group.ref,
			Summary:    fmt.Sprintf("%.0f InnoDB deadlocks occurred in the sampled window.", deadlocks),
			Guidance:   "Inspect transaction ordering, lock scope, indexes, and retry behavior using application/server diagnostics; do not disable deadlock detection as a remediation shortcut.",
			Confidence: 0.92,
		})
	}
	return out
}

type fullScanHeavyTableRule struct{}

func (fullScanHeavyTableRule) ID() finding.ID { return "mysql.full_scan_heavy_table" }
func (fullScanHeavyTableRule) Requires() []capability.Capability {
	return []capability.Capability{"schema.objects", "mysql.performance_schema"}
}
func (r fullScanHeavyTableRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	out := []finding.Finding{}
	for _, group := range sortedDeltaGroups(ctx.Deltas) {
		fullScanReads, ok := group.values["mysql.table.full_scan_rows"]
		if !ok || fullScanReads < 100000 {
			continue
		}
		estimateObservation, estimatedRows, okEstimate := firstNumber(ctx.Current, "mysql.table.estimated_rows", &group.ref)
		if !okEstimate || estimatedRows < 1000 {
			continue
		}
		severity := finding.Severity("warn")
		if fullScanReads >= 1000000 {
			severity = "critical"
		}
		out = append(out, finding.Finding{
			ID:         r.ID(),
			Title:      "Large table has heavy no-index read activity",
			Severity:   severity,
			Object:     group.ref,
			Evidence:   []signal.Observation{estimateObservation},
			Summary:    fmt.Sprintf("Approximately %.0f rows were read without an index in the sampled window; table row estimate is %.0f.", fullScanReads, estimatedRows),
			Guidance:   "Correlate with query digests and plans before adding indexes; full scans can be intentional for analytical workloads.",
			Confidence: 0.82,
		})
	}
	return out
}

func sortedDeltaGroups(deltas []signal.Delta) []*deltaGroup {
	grouped := groupDeltas(deltas)
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	ordered := make([]*deltaGroup, 0, len(keys))
	for _, key := range keys {
		ordered = append(ordered, grouped[key])
	}
	return ordered
}
