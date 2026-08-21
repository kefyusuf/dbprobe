package findings

import (
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestLockWaitLongUsesActualWaitDurationAndStableFindingIdentity(t *testing.T) {
	transaction := object.Ref{Kind: "mysql.transaction", ID: "ephemeral:123"}
	ctx := finding.AnalysisContext{Capabilities: capability.New("activity.transactions"), Current: []signal.Observation{
		numberRef("mysql.transaction.lock_wait_seconds", transaction, 55),
	}}
	got := lockWaitLongRule{}.Evaluate(ctx)
	assertFinding(t, got, "mysql.lock_wait_long", "critical")
	if got[0].Object.ID == transaction.ID {
		t.Fatal("finding used ephemeral transaction ID as suppression identity")
	}

	ctx.Current[0] = numberRef("mysql.transaction.lock_wait_seconds", transaction, 30)
	assertFinding(t, lockWaitLongRule{}.Evaluate(ctx), "mysql.lock_wait_long", "warn")

	ctx.Current[0] = numberRef("mysql.transaction.lock_wait_seconds", transaction, 29)
	if got := (lockWaitLongRule{}).Evaluate(ctx); len(got) != 0 {
		t.Fatalf("finding fired below threshold: %#v", got)
	}
}
