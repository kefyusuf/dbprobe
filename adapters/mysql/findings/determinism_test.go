package findings

import (
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestNoGoodIndexRuleEmitsFindingsInStableObjectOrder(t *testing.T) {
	ctx := finding.AnalysisContext{Deltas: []signal.Delta{
		{Key: "mysql.query.no_good_index_used", Object: object.Ref{Kind: "mysql.query", ID: "query-b"}, Delta: 20},
		{Key: "core.query.calls", Object: object.Ref{Kind: "mysql.query", ID: "query-a"}, Delta: 20},
		{Key: "core.query.calls", Object: object.Ref{Kind: "mysql.query", ID: "query-b"}, Delta: 20},
		{Key: "mysql.query.no_good_index_used", Object: object.Ref{Kind: "mysql.query", ID: "query-a"}, Delta: 20},
	}}

	for run := 0; run < 1000; run++ {
		got := (noGoodIndexRule{}).Evaluate(ctx)
		if len(got) != 2 {
			t.Fatalf("run=%d findings=%d want=2", run, len(got))
		}
		if got[0].Object.ID != "query-a" || got[1].Object.ID != "query-b" {
			t.Fatalf("run=%d finding order=%q,%q want=query-a,query-b", run, got[0].Object.ID, got[1].Object.ID)
		}
	}
}
