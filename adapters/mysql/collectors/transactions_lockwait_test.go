package collectors

import (
	"context"
	"strings"
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestTransactionCollectorEmitsActualLockWaitAge(t *testing.T) {
	q := &recordingQueryer{rows: [][]string{{"123", "77", "LOCK WAIT", "600", "42", "10", "55"}}}
	got, err := NewTransactions(q, 50).Collect(context.Background(), collector.Request{})
	if err != nil {
		t.Fatal(err)
	}
	assertOperationalNumber(t, got, "mysql.transaction.lock_wait_seconds", "mysql.transaction", "ephemeral:123", 55)
	for _, observation := range got {
		if observation.Key == "mysql.transaction.lock_wait_seconds" && observation.Unit != signal.UnitSeconds {
			t.Fatalf("lock wait unit = %q", observation.Unit)
		}
	}
	if !strings.Contains(strings.ToUpper(q.query), "TRX_WAIT_STARTED") {
		t.Fatalf("transaction collector does not use TRX_WAIT_STARTED: %s", q.query)
	}
}
