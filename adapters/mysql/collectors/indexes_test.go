package collectors

import (
	"context"
	"strings"
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestIndexCollectorEmitsStableSchemaMetadataAndReadCount(t *testing.T) {
	q := &recordingQueryer{rows: [][]string{{
		"shop", "orders", "idx_customer_created", "1", "customer_id,created_at", "0",
	}}}
	got, err := NewIndexes(q, "shop", 100).Collect(context.Background(), collector.Request{Phase: collector.PhaseSampleA})
	if err != nil {
		t.Fatal(err)
	}
	assertIndexNumber(t, got, "mysql.index.reads", 0)
	assertIndexText(t, got, "mysql.index.columns", "customer_id,created_at")
	assertIndexBool(t, got, "mysql.index.unique", false)
	assertIndexBool(t, got, "mysql.index.primary", false)
	if len(q.arguments) != 2 || q.arguments[0] != "shop" || q.arguments[1] != 100 {
		t.Fatalf("args = %#v", q.arguments)
	}
	if !strings.Contains(q.query, "information_schema.statistics") || !strings.Contains(q.query, "COUNT_READ") {
		t.Fatalf("unexpected index query: %s", q.query)
	}
}

func TestIndexCollectorMarksPrimaryAndClampsLimit(t *testing.T) {
	q := &recordingQueryer{rows: [][]string{{"shop", "orders", "PRIMARY", "0", "id", "100"}}}
	got, err := NewIndexes(q, "shop", 999).Collect(context.Background(), collector.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if q.arguments[1] != 200 {
		t.Fatalf("limit = %#v", q.arguments[1])
	}
	assertIndexBool(t, got, "mysql.index.unique", true)
	assertIndexBool(t, got, "mysql.index.primary", true)
}

func assertIndexNumber(t *testing.T, observations []signal.Observation, key signal.Key, want float64) {
	t.Helper()
	for _, observation := range observations {
		if observation.Key == key {
			value, ok := observation.Numeric()
			if !ok || value != want || observation.Object.Kind != "mysql.index" || observation.Object.ID != "shop.orders.idx_customer_created" {
				t.Fatalf("%s observation = %#v", key, observation)
			}
			return
		}
	}
	t.Fatalf("missing %s", key)
}

func assertIndexText(t *testing.T, observations []signal.Observation, key signal.Key, want string) {
	t.Helper()
	for _, observation := range observations {
		if observation.Key == key {
			if observation.Text == nil || *observation.Text != want {
				t.Fatalf("%s = %#v", key, observation.Text)
			}
			return
		}
	}
	t.Fatalf("missing %s", key)
}

func assertIndexBool(t *testing.T, observations []signal.Observation, key signal.Key, want bool) {
	t.Helper()
	for _, observation := range observations {
		if observation.Key == key {
			if observation.Boolean == nil || *observation.Boolean != want {
				t.Fatalf("%s = %#v", key, observation.Boolean)
			}
			return
		}
	}
	t.Fatalf("missing %s", key)
}
