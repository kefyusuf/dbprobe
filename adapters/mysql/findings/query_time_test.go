package findings

import (
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestQueryHighTotalTimeUsesSampleWindowIntensity(t *testing.T) {
	query := object.Ref{Kind: "mysql.query", ID: "shop:q"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("workload.query_summary"), Deltas: []signal.Delta{
		{Key: "core.query.calls", Object: query, Delta: 20, Exactness: signal.ExactnessSampled, WindowSeconds: 10},
		{Key: "mysql.query.total_latency_ms", Object: query, Delta: 25000, Exactness: signal.ExactnessSampled, WindowSeconds: 10},
	}}
	assertFinding(t, queryHighTotalTimeRule{}.Evaluate(ctx), "mysql.query_high_total_time", "critical")

	ctx.Deltas[1].Delta = 6000
	assertFinding(t, queryHighTotalTimeRule{}.Evaluate(ctx), "mysql.query_high_total_time", "warn")

	ctx.Deltas[1].Delta = 4000
	if got := (queryHighTotalTimeRule{}).Evaluate(ctx); len(got) != 0 {
		t.Fatalf("query total-time finding fired below intensity threshold: %#v", got)
	}
}
