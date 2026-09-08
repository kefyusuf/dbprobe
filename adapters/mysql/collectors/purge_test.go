package collectors

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestPurgeHistoryCollectorReadsDefaultOnHistoryMetric(t *testing.T) {
	q := &recordingQueryer{rows: [][]string{{"250000"}}}
	got, err := NewPurgeHistory(q, "instance-1").Collect(context.Background(), collector.Request{CollectedAt: time.Unix(1, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("observations=%#v", got)
	}
	observation := got[0]
	value, ok := observation.Numeric()
	if !ok || value != 250000 || observation.Key != "mysql.innodb.history_list_length" || observation.Exactness != signal.ExactnessScraped {
		t.Fatalf("observation=%#v", observation)
	}
	if observation.Object.Kind != "mysql.instance" || observation.Object.ID != "instance-1" {
		t.Fatalf("object=%#v", observation.Object)
	}
	if !strings.Contains(q.query, "INNODB_METRICS") || !strings.Contains(q.query, "trx_rseg_history_len") {
		t.Fatalf("query=%s", q.query)
	}
}
