package findings

import (
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestGroupDeltasReturnsStableObjectOrder(t *testing.T) {
	groups := groupDeltas([]signal.Delta{
		{Key: "mysql.query.no_good_index_used", Object: object.Ref{Kind: "mysql.query", ID: "query-b"}, Delta: 20},
		{Key: "core.query.calls", Object: object.Ref{Kind: "mysql.query", ID: "query-a"}, Delta: 20},
		{Key: "core.query.calls", Object: object.Ref{Kind: "mysql.query", ID: "query-b"}, Delta: 20},
		{Key: "mysql.query.no_good_index_used", Object: object.Ref{Kind: "mysql.query", ID: "query-a"}, Delta: 20},
	})
	if len(groups) != 2 {
		t.Fatalf("groups=%d want=2", len(groups))
	}
	if groups[0].ref.ID != "query-a" || groups[1].ref.ID != "query-b" {
		t.Fatalf("group order=%q,%q want=query-a,query-b", groups[0].ref.ID, groups[1].ref.ID)
	}
}
