package collectors

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type fixtureQueryer struct {
	rows [][]string
	err  error
}

func (q fixtureQueryer) QueryContext(context.Context, string, ...any) (Rows, error) {
	if q.err != nil {
		return nil, q.err
	}
	return &fixtureRows{rows: q.rows}, nil
}

type fixtureRows struct {
	rows [][]string
	pos  int
}

func (r *fixtureRows) Next() bool { return r.pos < len(r.rows) }
func (r *fixtureRows) Scan(dest ...any) error {
	if r.pos >= len(r.rows) || len(dest) != len(r.rows[r.pos]) {
		return fmt.Errorf("invalid fixture scan")
	}
	for i := range dest {
		value, ok := dest[i].(*string)
		if !ok {
			return fmt.Errorf("unexpected scan destination %d", i)
		}
		*value = r.rows[r.pos][i]
	}
	r.pos++
	return nil
}
func (r *fixtureRows) Close() error { return nil }
func (r *fixtureRows) Err() error   { return nil }

func TestHealthSnapshotMapsPortableAndMySQLSignals(t *testing.T) {
	q := fixtureQueryer{rows: [][]string{
		{"Threads_connected", "12"},
		{"Threads_running", "3"},
		{"Uptime", "86400"},
		{"max_connections", "151"},
		{"Ignored_future_metric", "99"},
	}}
	collectors := NewHealth(q, "instance-1")
	if len(collectors) != 2 {
		t.Fatalf("collectors = %d, want 2", len(collectors))
	}
	got, err := collectors[0].Collect(context.Background(), collector.Request{Phase: collector.PhaseSingle, CollectedAt: time.Unix(1, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	assertNumber(t, got, "core.connections.used", 12, signal.ExactnessScraped)
	assertNumber(t, got, "core.connections.limit", 151, signal.ExactnessScraped)
	assertNumber(t, got, "mysql.threads.running", 3, signal.ExactnessScraped)
	assertNumber(t, got, "mysql.server.uptime_seconds", 86400, signal.ExactnessScraped)
	for _, observation := range got {
		if observation.Key == "mysql.server.uptime_seconds" && observation.Unit != signal.UnitSeconds {
			t.Fatalf("uptime unit = %q", observation.Unit)
		}
	}
	if len(got) != 4 {
		t.Fatalf("observations = %d, want 4", len(got))
	}
}

func TestHealthCounterMarksCumulativeSignals(t *testing.T) {
	q := fixtureQueryer{rows: [][]string{
		{"Connections", "1000"},
		{"Created_tmp_disk_tables", "7"},
		{"Innodb_buffer_pool_read_requests", "50000"},
		{"Innodb_buffer_pool_reads", "50"},
		{"Innodb_row_lock_waits", "9"},
		{"Innodb_log_waits", "2"},
		{"Com_commit", "700"},
		{"Com_rollback", "4"},
	}}
	collectors := NewHealth(q, "instance-1")
	got, err := collectors[1].Collect(context.Background(), collector.Request{Phase: collector.PhaseSampleA, CollectedAt: time.Unix(2, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[signal.Key]float64{
		"mysql.connections.total":                1000,
		"mysql.temp.disk_tables":                 7,
		"mysql.innodb.buffer_pool.read_requests": 50000,
		"mysql.innodb.buffer_pool.reads":         50,
		"mysql.innodb.row_lock_waits":            9,
		"mysql.innodb.log_waits":                 2,
		"mysql.statements.commit":                700,
		"mysql.statements.rollback":              4,
	} {
		assertNumber(t, got, key, want, signal.ExactnessCumulative)
	}
}

func TestHealthCollectorReturnsQueryFailure(t *testing.T) {
	collectors := NewHealth(fixtureQueryer{err: fmt.Errorf("permission denied")}, "instance-1")
	if _, err := collectors[0].Collect(context.Background(), collector.Request{}); err == nil {
		t.Fatal("expected collector error")
	}
}

func assertNumber(t *testing.T, observations []signal.Observation, key signal.Key, want float64, exactness signal.Exactness) {
	t.Helper()
	for _, observation := range observations {
		if observation.Key != key {
			continue
		}
		got, ok := observation.Numeric()
		if !ok || got != want || observation.Exactness != exactness {
			t.Fatalf("%s = value:%v ok:%v exactness:%q", key, got, ok, observation.Exactness)
		}
		if observation.Object.Kind != "mysql.instance" || observation.Object.ID != "instance-1" {
			t.Fatalf("%s object = %#v", key, observation.Object)
		}
		return
	}
	t.Fatalf("missing observation %s", key)
}
