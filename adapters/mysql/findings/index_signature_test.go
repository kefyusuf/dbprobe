package findings

import (
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestRedundantIndexDoesNotCollapseDifferentPrefixOrDirection(t *testing.T) {
	short := object.Ref{Kind: "mysql.index", ID: "shop.people.idx_name_prefix"}
	long := object.Ref{Kind: "mysql.index", ID: "shop.people.idx_name_created"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("schema.indexes"), Current: []signal.Observation{
		textRef("mysql.index.table", short, "shop.people"),
		textRef("mysql.index.columns", short, "name"),
		textRef("mysql.index.signature", short, "8:col:name:10:A"),
		booleanRef("mysql.index.unique", short, false),
		booleanRef("mysql.index.primary", short, false),
		textRef("mysql.index.table", long, "shop.people"),
		textRef("mysql.index.columns", long, "name,created_at"),
		textRef("mysql.index.signature", long, "8:col:name:full:A;14:col:created_at:full:A"),
		booleanRef("mysql.index.unique", long, false),
		booleanRef("mysql.index.primary", long, false),
	}}
	if got := (redundantIndexRule{}).Evaluate(ctx); len(got) != 0 {
		t.Fatalf("different prefix length was treated as redundant: %#v", got)
	}
}
