package findings

import (
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestBlockingChainUsesWaitGraphWithoutEphemeralFindingIdentity(t *testing.T) {
	table := object.Ref{Kind: "mysql.table", ID: "shop.orders"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("locking.wait_graph"), Current: []signal.Observation{
		textRef("mysql.lock_wait.edge", table, "A->B"),
		textRef("mysql.lock_wait.edge", table, "B->C"),
	}}
	got := blockingChainRule{}.Evaluate(ctx)
	assertFinding(t, got, "mysql.blocking_chain", "warn")
	if got[0].Object.ID == "A" || got[0].Object.ID == "B" || got[0].Object.ID == "C" {
		t.Fatal("finding uses ephemeral transaction id")
	}

	ctx.Current = append(ctx.Current, textRef("mysql.lock_wait.edge", table, "C->D"))
	assertFinding(t, blockingChainRule{}.Evaluate(ctx), "mysql.blocking_chain", "critical")

	ctx.Current = ctx.Current[:1]
	if got := (blockingChainRule{}).Evaluate(ctx); len(got) != 0 {
		t.Fatalf("single edge should not be a chain finding: %#v", got)
	}
}
