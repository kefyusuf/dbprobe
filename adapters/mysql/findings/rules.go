package findings

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func Rules() []finding.Rule {
	return []finding.Rule{
		connectionSaturationRule{},
		longTransactionRule{},
		queryFullScanRule{},
		queryAmplificationRule{},
		unusedIndexRule{},
		redundantIndexRule{},
		bufferPoolHitRule{},
		replicationStoppedRule{},
		replicationErrorRule{},
		lockContentionRule{},
	}
}

type connectionSaturationRule struct{}

func (connectionSaturationRule) ID() finding.ID { return "core.connection_saturation" }
func (connectionSaturationRule) Requires() []capability.Capability {
	return []capability.Capability{"mysql.performance_schema"}
}
func (r connectionSaturationRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	usedObs, used, okUsed := firstNumber(ctx.Current, "core.connections.used", nil)
	_, limit, okLimit := firstNumber(ctx.Current, "core.connections.limit", nil)
	if !okUsed || !okLimit || limit <= 0 {
		return nil
	}
	ratio := used / limit
	severity := finding.Severity("")
	switch {
	case ratio >= 0.95:
		severity = "critical"
	case ratio >= 0.80:
		severity = "warn"
	default:
		return nil
	}
	return []finding.Finding{{ID: r.ID(), Title: "Connection capacity is saturated", Severity: severity, Object: usedObs.Object, Evidence: []signal.Observation{usedObs}, Summary: fmt.Sprintf("%.0f of %.0f configured connections are in use (%.1f%%).", used, limit, ratio*100), Guidance: "Investigate connection leaks, pool sizing, long-running work, and max_connections together before changing limits.", Confidence: 0.95}}
}

type longTransactionRule struct{}

func (longTransactionRule) ID() finding.ID { return "mysql.long_transaction" }
func (longTransactionRule) Requires() []capability.Capability {
	return []capability.Capability{"activity.transactions"}
}
func (r longTransactionRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	var out []finding.Finding
	for _, obs := range ctx.Current {
		if obs.Key != "mysql.transaction.age_seconds" || obs.Number == nil {
			continue
		}
		age := *obs.Number
		var severity finding.Severity
		switch {
		case age >= 1800:
			severity = "critical"
		case age >= 300:
			severity = "warn"
		default:
			continue
		}
		out = append(out, finding.Finding{ID: r.ID(), Title: "Long-running transaction", Severity: severity, Object: object.Ref{Kind: "mysql.transaction_class", ID: "long-running"}, Evidence: []signal.Observation{obs}, Summary: fmt.Sprintf("A transaction has been open for %.0f seconds.", age), Guidance: "Identify the owning application/session and shorten the transaction boundary. Avoid terminating sessions automatically.", Confidence: 0.95})
	}
	return out
}

type queryFullScanRule struct{}

func (queryFullScanRule) ID() finding.ID { return "mysql.query_full_scan_heavy" }
func (queryFullScanRule) Requires() []capability.Capability {
	return []capability.Capability{"workload.query_summary"}
}
func (r queryFullScanRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	grouped := groupDeltas(ctx.Deltas)
	var out []finding.Finding
	for _, g := range grouped {
		calls, okCalls := g.values["core.query.calls"]
		noIndex, okNoIndex := g.values["mysql.query.no_index_used"]
		if !okCalls || !okNoIndex || calls < 10 || noIndex < 5 || noIndex/calls < 0.50 {
			continue
		}
		out = append(out, finding.Finding{ID: r.ID(), Title: "Query frequently executes without an index", Severity: "warn", Object: g.ref, Summary: fmt.Sprintf("%.0f of %.0f executions in the sampled window reported no index usage.", noIndex, calls), Guidance: "Review the query shape, table size, selectivity, and EXPLAIN plan before adding an index; small-table scans can be intentional.", Confidence: 0.85})
	}
	return out
}

type queryAmplificationRule struct{}

func (queryAmplificationRule) ID() finding.ID { return "mysql.query_rows_examined_amplification" }
func (queryAmplificationRule) Requires() []capability.Capability {
	return []capability.Capability{"workload.query_summary"}
}
func (r queryAmplificationRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	grouped := groupDeltas(ctx.Deltas)
	var out []finding.Finding
	for _, g := range grouped {
		examined, okExamined := g.values["mysql.query.rows_examined"]
		sent, okSent := g.values["mysql.query.rows_sent"]
		if !okExamined || !okSent || examined < 10000 {
			continue
		}
		denominator := sent
		if denominator < 1 {
			denominator = 1
		}
		ratio := examined / denominator
		var severity finding.Severity
		switch {
		case ratio >= 1000:
			severity = "critical"
		case ratio >= 100:
			severity = "warn"
		default:
			continue
		}
		out = append(out, finding.Finding{ID: r.ID(), Title: "Query examines far more rows than it returns", Severity: severity, Object: g.ref, Summary: fmt.Sprintf("The sampled workload examined %.0f rows for %.0f rows sent (%.0fx amplification).", examined, sent, ratio), Guidance: "Inspect predicates, join order, statistics, and candidate indexes with a non-executing EXPLAIN before changing schema.", Confidence: 0.90})
	}
	return out
}

type unusedIndexRule struct{}

func (unusedIndexRule) ID() finding.ID { return "mysql.unused_index" }
func (unusedIndexRule) Requires() []capability.Capability {
	return []capability.Capability{"schema.indexes", "mysql.performance_schema"}
}
func (r unusedIndexRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	_, uptime, ok := firstNumber(ctx.Current, "mysql.server.uptime_seconds", nil)
	if !ok || uptime < 3600 {
		return nil
	}
	states := indexStates(ctx.Current)
	ids := sortedIndexIDs(states)
	out := make([]finding.Finding, 0)
	for _, id := range ids {
		s := states[id]
		if !s.hasReads || s.reads != 0 || !s.hasPrimary || s.primary || !s.hasUnique || s.unique {
			continue
		}
		out = append(out, finding.Finding{ID: r.ID(), Title: "Index has no observed reads", Severity: "info", Object: s.ref, Summary: fmt.Sprintf("No index reads were observed while server uptime is %.1f hours.", uptime/3600), Guidance: "Treat this as a review candidate only. Confirm representative workload, Performance Schema reset history, constraints, and application usage before considering removal.", Confidence: 0.60})
	}
	return out
}

type redundantIndexRule struct{}

func (redundantIndexRule) ID() finding.ID { return "mysql.redundant_index" }
func (redundantIndexRule) Requires() []capability.Capability {
	return []capability.Capability{"schema.indexes"}
}
func (r redundantIndexRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	states := indexStates(ctx.Current)
	ids := sortedIndexIDs(states)
	var out []finding.Finding
	for _, shortID := range ids {
		a := states[shortID]
		if !a.completeForRedundancy() || a.primary || a.unique || a.columns == "" || a.table == "" {
			continue
		}
		for _, longID := range ids {
			if shortID == longID {
				continue
			}
			b := states[longID]
			if !b.completeForRedundancy() || b.primary || b.unique || a.table != b.table {
				continue
			}
			if len(a.columns) >= len(b.columns) {
				continue
			}
			if b.columns == a.columns || strings.HasPrefix(b.columns, a.columns+",") {
				out = append(out, finding.Finding{ID: r.ID(), Title: "Index may be redundant", Severity: "info", Object: a.ref, Summary: fmt.Sprintf("Index columns (%s) are a left-prefix of %s (%s).", a.columns, b.ref.ID, b.columns), Guidance: "Review uniqueness, ordering, prefix lengths, optimizer plans, write cost, and production workload before removing either index.", Confidence: 0.70})
				break
			}
		}
	}
	return out
}

type bufferPoolHitRule struct{}

func (bufferPoolHitRule) ID() finding.ID { return "mysql.buffer_pool_hit_low" }
func (bufferPoolHitRule) Requires() []capability.Capability {
	return []capability.Capability{"storage.cache"}
}
func (r bufferPoolHitRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	grouped := groupDeltas(ctx.Deltas)
	var out []finding.Finding
	for _, g := range grouped {
		requests, okReq := g.values["mysql.innodb.buffer_pool.read_requests"]
		reads, okReads := g.values["mysql.innodb.buffer_pool.reads"]
		if !okReq || !okReads || requests < 1000 || requests <= 0 {
			continue
		}
		missRatio := reads / requests
		hitRatio := 1 - missRatio
		var severity finding.Severity
		switch {
		case hitRatio < 0.70:
			severity = "critical"
		case hitRatio < 0.95:
			severity = "warn"
		default:
			continue
		}
		out = append(out, finding.Finding{ID: r.ID(), Title: "InnoDB buffer pool hit ratio is low", Severity: severity, Object: g.ref, Summary: fmt.Sprintf("Sampled buffer-pool hit ratio is %.1f%% across %.0f read requests.", hitRatio*100, requests), Guidance: "Correlate with working-set size, memory pressure, scans, and storage latency before changing innodb_buffer_pool_size.", Confidence: 0.85})
	}
	return out
}

type replicationStoppedRule struct{}

func (replicationStoppedRule) ID() finding.ID { return "mysql.replication_stopped" }
func (replicationStoppedRule) Requires() []capability.Capability {
	return []capability.Capability{"replication.status"}
}
func (r replicationStoppedRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	states := map[string]*replicationChannel{}
	for _, obs := range ctx.Current {
		if obs.Object.Kind != "mysql.replication_channel" || obs.Text == nil {
			continue
		}
		s := states[obs.Object.ID]
		if s == nil {
			s = &replicationChannel{ref: obs.Object}
			states[obs.Object.ID] = s
		}
		switch obs.Key {
		case "mysql.replication.receiver_state":
			s.receiver = *obs.Text
			s.hasReceiver = true
		case "mysql.replication.applier_state":
			s.applier = *obs.Text
			s.hasApplier = true
		}
	}
	ids := make([]string, 0, len(states))
	for id := range states {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var out []finding.Finding
	for _, id := range ids {
		s := states[id]
		if !(s.hasReceiver || s.hasApplier) {
			continue
		}
		if (!s.hasReceiver || strings.ToUpper(s.receiver) != "ON") || (!s.hasApplier || strings.ToUpper(s.applier) != "ON") {
			out = append(out, finding.Finding{ID: r.ID(), Title: "Replication channel is not fully running", Severity: "critical", Object: s.ref, Summary: fmt.Sprintf("Receiver state=%q, applier state=%q.", s.receiver, s.applier), Guidance: "Inspect receiver/applier error codes, connectivity, source availability, and replication configuration before restarting components.", Confidence: 0.95})
		}
	}
	return out
}

type replicationErrorRule struct{}

func (replicationErrorRule) ID() finding.ID { return "mysql.replication_error" }
func (replicationErrorRule) Requires() []capability.Capability {
	return []capability.Capability{"replication.status"}
}
func (r replicationErrorRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	var out []finding.Finding
	for _, obs := range ctx.Current {
		if obs.Key != "mysql.replication.error_code" || obs.Number == nil || *obs.Number == 0 {
			continue
		}
		out = append(out, finding.Finding{ID: r.ID(), Title: "Replication component reports an error", Severity: "critical", Object: obs.Object, Evidence: []signal.Observation{obs}, Summary: fmt.Sprintf("Replication error code %.0f is present.", *obs.Number), Guidance: "Resolve the numeric replication error using server logs and source/replica state. dbprobe intentionally does not collect potentially sensitive error-message text.", Confidence: 0.98})
	}
	return out
}

type lockContentionRule struct{}

func (lockContentionRule) ID() finding.ID { return "mysql.lock_wait_contention" }
func (lockContentionRule) Requires() []capability.Capability {
	return []capability.Capability{"locking.wait_graph"}
}
func (r lockContentionRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	totals := map[string]float64{}
	refs := map[string]object.Ref{}
	for _, obs := range ctx.Current {
		if obs.Key != "mysql.lock_wait.count" || obs.Number == nil || obs.Object.Kind != "mysql.table" {
			continue
		}
		totals[obs.Object.ID] += *obs.Number
		refs[obs.Object.ID] = obs.Object
	}
	ids := make([]string, 0, len(totals))
	for id := range totals {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var out []finding.Finding
	for _, id := range ids {
		n := totals[id]
		if n < 3 {
			continue
		}
		sev := finding.Severity("warn")
		if n >= 10 {
			sev = "critical"
		}
		out = append(out, finding.Finding{ID: r.ID(), Title: "Table has concurrent lock waits", Severity: sev, Object: refs[id], Summary: fmt.Sprintf("%.0f lock-wait edges currently involve this table.", n), Guidance: "Inspect blocking transactions and transaction boundaries; do not automatically kill blockers without application context.", Confidence: 0.90})
	}
	return out
}

type deltaGroup struct {
	ref    object.Ref
	values map[signal.Key]float64
}

func groupDeltas(deltas []signal.Delta) map[string]*deltaGroup {
	out := map[string]*deltaGroup{}
	for _, d := range deltas {
		key := d.Object.Kind + "|" + d.Object.ID
		g := out[key]
		if g == nil {
			g = &deltaGroup{ref: d.Object, values: map[signal.Key]float64{}}
			out[key] = g
		}
		g.values[d.Key] = d.Delta
	}
	return out
}

func firstNumber(obs []signal.Observation, key signal.Key, ref *object.Ref) (signal.Observation, float64, bool) {
	for _, o := range obs {
		if o.Key != key || o.Number == nil {
			continue
		}
		if ref != nil && (o.Object != *ref) {
			continue
		}
		return o, *o.Number, true
	}
	return signal.Observation{}, 0, false
}

type indexState struct {
	ref        object.Ref
	reads      float64
	hasReads   bool
	primary    bool
	hasPrimary bool
	unique     bool
	hasUnique  bool
	table      string
	hasTable   bool
	columns    string
	hasColumns bool
}

func (s *indexState) completeForRedundancy() bool {
	return s.hasPrimary && s.hasUnique && s.hasTable && s.hasColumns
}
func indexStates(obs []signal.Observation) map[string]*indexState {
	m := map[string]*indexState{}
	for _, o := range obs {
		if o.Object.Kind != "mysql.index" {
			continue
		}
		s := m[o.Object.ID]
		if s == nil {
			s = &indexState{ref: o.Object}
			m[o.Object.ID] = s
		}
		switch o.Key {
		case "mysql.index.reads":
			if o.Number != nil {
				s.reads = *o.Number
				s.hasReads = true
			}
		case "mysql.index.primary":
			if o.Boolean != nil {
				s.primary = *o.Boolean
				s.hasPrimary = true
			}
		case "mysql.index.unique":
			if o.Boolean != nil {
				s.unique = *o.Boolean
				s.hasUnique = true
			}
		case "mysql.index.table":
			if o.Text != nil {
				s.table = *o.Text
				s.hasTable = true
			}
		case "mysql.index.columns":
			if o.Text != nil {
				s.columns = *o.Text
				s.hasColumns = true
			}
		}
	}
	return m
}
func sortedIndexIDs(m map[string]*indexState) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

type replicationChannel struct {
	ref                     object.Ref
	receiver, applier       string
	hasReceiver, hasApplier bool
}
