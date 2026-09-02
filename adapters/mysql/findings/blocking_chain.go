package findings

import (
	"fmt"
	"strings"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func BlockingChainRules() []finding.Rule { return []finding.Rule{blockingChainRule{}} }

type blockingChainRule struct{}

func (blockingChainRule) ID() finding.ID { return "mysql.blocking_chain" }
func (blockingChainRule) Requires() []capability.Capability {
	return []capability.Capability{"locking.wait_graph"}
}
func (r blockingChainRule) Evaluate(ctx finding.AnalysisContext) []finding.Finding {
	adjacency := map[string][]string{}
	evidence := []signal.Observation{}
	for _, observation := range ctx.Current {
		if observation.Key != "mysql.lock_wait.edge" || observation.Text == nil {
			continue
		}
		parts := strings.Split(*observation.Text, "->")
		if len(parts) != 2 {
			continue
		}
		requesting := strings.TrimSpace(parts[0])
		blocking := strings.TrimSpace(parts[1])
		if requesting == "" || blocking == "" {
			continue
		}
		adjacency[requesting] = append(adjacency[requesting], blocking)
		evidence = append(evidence, observation)
	}

	longest := 0
	for node := range adjacency {
		if depth := blockingDepth(node, adjacency, map[string]bool{}); depth > longest {
			longest = depth
		}
	}
	if longest < 2 {
		return nil
	}

	severity := finding.Severity("warn")
	if longest >= 3 {
		severity = "critical"
	}
	return []finding.Finding{{
		ID:         r.ID(),
		Title:      "Multi-transaction blocking chain detected",
		Severity:   severity,
		Object:     object.Ref{Kind: "mysql.lock_graph", ID: "blocking-chain"},
		Evidence:   evidence,
		Summary:    fmt.Sprintf("The current InnoDB wait-for graph contains a blocking chain at least %d edges deep.", longest),
		Guidance:   "Inspect the involved transaction boundaries and stable table/index evidence. Do not use ephemeral transaction IDs as long-lived suppression keys or automatically kill blockers.",
		Confidence: 0.90,
	}}
}

func blockingDepth(node string, adjacency map[string][]string, path map[string]bool) int {
	if path[node] {
		return 0
	}
	path[node] = true
	defer delete(path, node)
	best := 0
	for _, next := range adjacency[node] {
		depth := 1 + blockingDepth(next, adjacency, path)
		if depth > best {
			best = depth
		}
	}
	return best
}
