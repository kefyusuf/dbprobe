package findings

import (
	"fmt"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func LockWaitRules() []finding.Rule {
	return []finding.Rule{lockWaitLongRule{}}
}

type lockWaitLongRule struct{}

func (lockWaitLongRule) ID() finding.ID { return "mysql.lock_wait_long" }
func (lockWaitLongRule) Requires() []capability.Capability {
	return []capability.Capability{"activity.transactions"}
}
func (r lockWaitLongRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	out := []finding.Finding{}
	for _, observation := range ctx.Current {
		if observation.Key != "mysql.transaction.lock_wait_seconds" || observation.Number == nil {
			continue
		}
		seconds := *observation.Number
		var severity finding.Severity
		switch {
		case seconds >= 50:
			severity = "critical"
		case seconds >= 30:
			severity = "warn"
		default:
			continue
		}
		out = append(out, finding.Finding{
			ID:         r.ID(),
			Title:      "Transaction has a prolonged InnoDB row-lock wait",
			Severity:   severity,
			Object:     object.Ref{Kind: "mysql.transaction_class", ID: "long-lock-wait"},
			Evidence:   []signal.Observation{observation},
			Summary:    fmt.Sprintf("An InnoDB transaction has been waiting on a row lock for %.0f seconds.", seconds),
			Guidance:   "Inspect the blocking transaction, lock graph, transaction boundaries, and application retry/timeout behavior. Do not automatically kill the blocker without application context.",
			Confidence: 0.95,
		})
	}
	return out
}
