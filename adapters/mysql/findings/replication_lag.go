package findings

import (
	"fmt"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func ReplicationLagRules() []finding.Rule { return []finding.Rule{replicationApplyLagRule{}} }

type replicationApplyLagRule struct{}

func (replicationApplyLagRule) ID() finding.ID { return "mysql.replication_apply_lag" }
func (replicationApplyLagRule) Requires() []capability.Capability {
	return []capability.Capability{"replication.status"}
}
func (r replicationApplyLagRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	out := []finding.Finding{}
	for _, observation := range ctx.Current {
		if observation.Key != "mysql.replication.active_apply_lag_seconds" || observation.Number == nil {
			continue
		}
		seconds := *observation.Number
		var severity finding.Severity
		switch {
		case seconds >= 120:
			severity = "critical"
		case seconds >= 30:
			severity = "warn"
		default:
			continue
		}
		out = append(out, finding.Finding{
			ID:       r.ID(),
			Title:    "Replica is applying a transaction well behind its source commit time",
			Severity: severity,
			Object:   observation.Object,
			Evidence: []signal.Observation{observation},
			Summary: fmt.Sprintf(
				"The actively applied transaction is %.0f seconds behind its original source commit timestamp after configured delayed-replication time is removed.",
				seconds,
			),
			Guidance:   "Inspect worker retries, transaction apply duration, replica CPU/IO pressure, source write bursts, and replication parallelism. Idle channels intentionally do not emit this signal; verify source/replica clock synchronization before treating small differences as exact lag.",
			Confidence: 0.88,
		})
	}
	return out
}
