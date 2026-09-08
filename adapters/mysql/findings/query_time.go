package findings

import (
	"fmt"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func QueryTimeRules() []finding.Rule {
	return []finding.Rule{queryHighTotalTimeRule{}}
}

type queryHighTotalTimeRule struct{}

func (queryHighTotalTimeRule) ID() finding.ID { return "mysql.query_high_total_time" }
func (queryHighTotalTimeRule) Requires() []capability.Capability {
	return []capability.Capability{"workload.query_summary"}
}

func (r queryHighTotalTimeRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	grouped := groupDeltas(ctx.Deltas)
	out := []finding.Finding{}
	for _, metric := range ctx.Deltas {
		if metric.Key != "mysql.query.total_latency_ms" || metric.Exactness != signal.ExactnessSampled || metric.WindowSeconds <= 0 || metric.Delta < 1000 {
			continue
		}
		group := grouped[metric.Object.Kind+"|"+metric.Object.ID]
		if group == nil {
			continue
		}
		calls, ok := group.values["core.query.calls"]
		if !ok || calls < 5 {
			continue
		}

		intensity := metric.Delta / (metric.WindowSeconds * 1000)
		var severity finding.Severity
		switch {
		case intensity >= 2.0:
			severity = "critical"
		case intensity >= 0.5:
			severity = "warn"
		default:
			continue
		}

		out = append(out, finding.Finding{
			ID:       r.ID(),
			Title:    "Query contributes high total statement time",
			Severity: severity,
			Object:   metric.Object,
			Summary: fmt.Sprintf(
				"The query accumulated %.2fs of statement time across %.0f calls in a %.2fs sample (%.2f database-seconds per wall-second).",
				metric.Delta/1000,
				calls,
				metric.WindowSeconds,
				intensity,
			),
			Guidance:   "Prioritize this query by total database-time contribution, then review its normalized shape, latency, row amplification, index signals, and a plan-only EXPLAIN before changing schema or configuration.",
			Confidence: 0.90,
		})
	}
	return out
}
