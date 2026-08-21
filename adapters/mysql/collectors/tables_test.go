package collectors

import (
	"context"
	"strings"
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestTableCollectorEmitsEstimatedRowsAndNoIndexReads(t *testing.T) {
	q := &recordingQueryer{rows: [][]string{{"shop", "orders", "250000", "1200000"}}}
	got, err := NewTables(q, "shop", 100).Collect(context.Background(), collector.Request{Phase: collector.PhaseSampleA})
	if err != nil {
		t.Fatal(err)
	}
	assertTableNumber(t, got, "mysql.table.estimated_rows", 250000, signal.ExactnessEstimated)
	assertTableNumber(t, got, "mysql.table.full_scan_rows", 1200000, signal.ExactnessCumulative)
	if len(q.arguments) != 2 || q.arguments[0] != "shop" || q.arguments[1] != 100 {
		t.Fatalf("args = %#v", q.arguments)
	}
	if !strings.Contains(q.query, "INDEX_NAME IS NULL") || !strings.Contains(q.query, "COUNT_READ") {
		t.Fatalf("query does not isolate no-index reads: %s", q.query)
	}
}

func TestTableCollectorClampsLimit(t *testing.T) {
	q := &recordingQueryer{}
	_, err := NewTables(q, "shop", 999).Collect(context.Background(), collector.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if q.arguments[1] != 200 {
		t.Fatalf("limit = %#v", q.arguments[1])
	}
}

func assertTableNumber(t *testing.T, observations []signal.Observation, key signal.Key, want float64, exactness signal.Exactness) {
	t.Helper()
	for _, observation := range observations {
		if observation.Key == key {
			value, ok := observation.Numeric()
			if !ok || value != want || observation.Exactness != exactness || observation.Object.Kind != "mysql.table" || observation.Object.ID != "shop.orders" {
				t.Fatalf("%s observation = %#v", key, observation)
			}
			return
		}
	}
	t.Fatalf("missing %s", key)
}
