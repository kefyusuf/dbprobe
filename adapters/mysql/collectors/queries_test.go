package collectors

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type recordingQueryer struct {
	rows      [][]string
	query     string
	arguments []any
}

func (q *recordingQueryer) QueryContext(_ context.Context, query string, args ...any) (Rows, error) {
	q.query = query
	q.arguments = append([]any(nil), args...)
	return &fixtureRows{rows: q.rows}, nil
}

func TestQueryCollectorMapsNormalizedDigestMetrics(t *testing.T) {
	q := &recordingQueryer{rows: [][]string{{
		"shop",
		"ABC123",
		"SELECT * FROM orders WHERE customer_id = ?",
		"25",
		"25000000000",
		"1000",
		"25",
		"8",
		"2",
		"3",
		"1",
		"4",
	}}}
	c := NewQueries(q, "shop", 20)
	got, err := c.Collect(context.Background(), collector.Request{Phase: collector.PhaseSampleA, CollectedAt: time.Unix(3, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}

	assertQueryNumber(t, got, "core.query.calls", 25)
	assertQueryNumber(t, got, "mysql.query.total_latency_ms", 25)
	assertQueryNumber(t, got, "mysql.query.rows_examined", 1000)
	assertQueryNumber(t, got, "mysql.query.rows_sent", 25)
	assertQueryNumber(t, got, "mysql.query.no_index_used", 8)
	assertQueryNumber(t, got, "mysql.query.no_good_index_used", 2)
	assertQueryNumber(t, got, "mysql.query.temp_disk_tables", 3)
	assertQueryNumber(t, got, "mysql.query.errors", 1)
	assertQueryNumber(t, got, "mysql.query.warnings", 4)
	assertQueryText(t, got, "SELECT * FROM orders WHERE customer_id = ?")

	if !strings.Contains(q.query, "SCHEMA_NAME = ?") || !strings.Contains(q.query, "LIMIT ?") {
		t.Fatalf("query is not scoped/bounded: %s", q.query)
	}
	if len(q.arguments) != 2 || q.arguments[0] != "shop" || q.arguments[1] != 20 {
		t.Fatalf("query args = %#v", q.arguments)
	}
}

func TestQueryCollectorClampsLimitAndSkipsNullDigestRowsBySQL(t *testing.T) {
	q := &recordingQueryer{}
	c := NewQueries(q, "shop", 500)
	_, err := c.Collect(context.Background(), collector.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(q.arguments) != 2 || q.arguments[1] != 50 {
		t.Fatalf("limit not clamped: %#v", q.arguments)
	}
	if !strings.Contains(q.query, "DIGEST IS NOT NULL") {
		t.Fatalf("digest overflow/sentinel rows are not excluded: %s", q.query)
	}
}

func TestQueryCollectorBoundsDigestText(t *testing.T) {
	long := strings.Repeat("x", 3000)
	q := &recordingQueryer{rows: [][]string{{"shop", "ABC123", long, "1", "1", "0", "0", "0", "0", "0", "0", "0"}}}
	got, err := NewQueries(q, "shop", 20).Collect(context.Background(), collector.Request{})
	if err != nil {
		t.Fatal(err)
	}
	for _, observation := range got {
		if observation.Key == "mysql.query.digest_text" {
			if observation.Text == nil || len(*observation.Text) > 2048 {
				t.Fatalf("digest text length = %d", len(*observation.Text))
			}
			return
		}
	}
	t.Fatal("missing digest text observation")
}

func assertQueryNumber(t *testing.T, observations []signal.Observation, key signal.Key, want float64) {
	t.Helper()
	for _, observation := range observations {
		if observation.Key != key {
			continue
		}
		value, ok := observation.Numeric()
		if !ok || value != want || observation.Exactness != signal.ExactnessCumulative {
			t.Fatalf("%s = %v, %v, %q", key, value, ok, observation.Exactness)
		}
		if observation.Object.Kind != "mysql.query" || observation.Object.ID != "shop:ABC123" {
			t.Fatalf("object = %#v", observation.Object)
		}
		return
	}
	t.Fatalf("missing %s", key)
}

func assertQueryText(t *testing.T, observations []signal.Observation, want string) {
	t.Helper()
	for _, observation := range observations {
		if observation.Key != "mysql.query.digest_text" {
			continue
		}
		if observation.Text == nil || *observation.Text != want || observation.Sensitivity != signal.SensitivityQueryShape {
			t.Fatalf("digest text = %#v sensitivity=%q", observation.Text, observation.Sensitivity)
		}
		return
	}
	t.Fatal("missing digest text")
}
