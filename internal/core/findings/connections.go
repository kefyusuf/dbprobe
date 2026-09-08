package findings

import (
	"fmt"
	"sort"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type connectionSaturationRule struct{}

func (connectionSaturationRule) ID() finding.ID { return "core.connection_saturation" }
func (connectionSaturationRule) Requires() []capability.Capability {
	return []capability.Capability{}
}

func (r connectionSaturationRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	type state struct {
		ref               object.Ref
		used, limit       float64
		usedObs, limitObs signal.Observation
		hasUsed, hasLimit bool
	}

	states := map[string]*state{}
	for _, observation := range ctx.Current {
		if observation.Number == nil || (observation.Key != "core.connections.used" && observation.Key != "core.connections.limit") {
			continue
		}
		key := observation.Object.Kind + "|" + observation.Object.ID
		s := states[key]
		if s == nil {
			s = &state{ref: observation.Object}
			states[key] = s
		}
		switch observation.Key {
		case "core.connections.used":
			s.used, s.usedObs, s.hasUsed = *observation.Number, observation, true
		case "core.connections.limit":
			s.limit, s.limitObs, s.hasLimit = *observation.Number, observation, true
		}
	}

	ids := make([]string, 0, len(states))
	for id := range states {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	out := make([]finding.Finding, 0)
	for _, id := range ids {
		s := states[id]
		if !s.hasUsed || !s.hasLimit || s.limit <= 0 || s.used < 0 {
			continue
		}
		ratio := s.used / s.limit
		var severity finding.Severity
		switch {
		case ratio >= 0.95:
			severity = "critical"
		case ratio >= 0.85:
			severity = "warn"
		default:
			continue
		}
		out = append(out, finding.Finding{
			ID:         r.ID(),
			Title:      "Connection capacity is saturated",
			Severity:   severity,
			Object:     s.ref,
			Evidence:   []signal.Observation{s.usedObs, s.limitObs},
			Summary:    fmt.Sprintf("%.0f of %.0f configured connections are in use (%.1f%%).", s.used, s.limit, ratio*100),
			Guidance:   "Investigate connection leaks, pool sizing, long-running work, and the configured connection limit together before changing capacity.",
			Confidence: 0.95,
		})
	}
	return out
}
