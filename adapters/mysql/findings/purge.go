package findings

import (
	"fmt"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func PurgeRules() []finding.Rule { return []finding.Rule{purgeLagRule{}} }

type purgeLagRule struct{}

func (purgeLagRule) ID() finding.ID { return "mysql.innodb_purge_lag" }
func (purgeLagRule) Requires() []capability.Capability {
	return []capability.Capability{"mysql.innodb_metrics"}
}
func (r purgeLagRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	out := []finding.Finding{}
	for _, observation := range ctx.Current {
		if observation.Key != "mysql.innodb.history_list_length" || observation.Number == nil {
			continue
		}
		length := *observation.Number
		var severity finding.Severity
		switch {
		case length >= 1000000:
			severity = "critical"
		case length >= 100000:
			severity = "warn"
		default:
			continue
		}
		out = append(out, finding.Finding{
			ID:         r.ID(),
			Title:      "InnoDB purge history list is large",
			Severity:   severity,
			Object:     observation.Object,
			Evidence:   []signal.Observation{observation},
			Summary:    fmt.Sprintf("InnoDB history list length is approximately %.0f undo records awaiting purge eligibility/progress.", length),
			Guidance:   "Correlate with long-running consistent-read transactions, write-heavy workload, purge-thread throughput, undo growth, and innodb_max_purge_lag policy. Do not tune purge limits without confirming the blocking workload.",
			Confidence: 0.90,
		})
	}
	return out
}
