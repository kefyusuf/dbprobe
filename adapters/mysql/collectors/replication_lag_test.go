package collectors

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestReplicationLagCollectorEmitsActiveExcessApplyLagOnly(t *testing.T) {
	q := &recordingQueryer{rows: [][]string{{"default", "45.5", "10"}}}
	got, err := NewReplicationLag(q, 100).Collect(context.Background(), collector.Request{CollectedAt: time.Unix(1, 0).UTC()})
	if err != nil {
		t.Fatal(err)
	}
	assertReplicationLagNumber(t, got, "mysql.replication.active_apply_lag_seconds", 45.5)
	assertReplicationLagNumber(t, got, "mysql.replication.desired_delay_seconds", 10)
	if !strings.Contains(q.query, "APPLYING_TRANSACTION") || !strings.Contains(q.query, "DESIRED_DELAY") {
		t.Fatalf("query does not model active apply and desired delay: %s", q.query)
	}
	if strings.Contains(q.query, "LAST_APPLIED_TRANSACTION") {
		t.Fatalf("query uses idle-unsafe last-applied timestamp: %s", q.query)
	}
}

func TestReplicationLagCollectorReturnsNoObservationWhenIdle(t *testing.T) {
	q := &recordingQueryer{}
	got, err := NewReplicationLag(q, 100).Collect(context.Background(), collector.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("idle observations = %#v", got)
	}
}

func TestReplicationLagCollectorClampsLimit(t *testing.T) {
	q := &recordingQueryer{}
	if _, err := NewReplicationLag(q, 999).Collect(context.Background(), collector.Request{}); err != nil {
		t.Fatal(err)
	}
	if len(q.arguments) != 1 || q.arguments[0] != 200 {
		t.Fatalf("args=%#v", q.arguments)
	}
}

func assertReplicationLagNumber(t *testing.T, observations []signal.Observation, key signal.Key, want float64) {
	t.Helper()
	for _, observation := range observations {
		if observation.Key != key {
			continue
		}
		value, ok := observation.Numeric()
		if !ok || value != want || observation.Unit != signal.UnitSeconds || observation.Exactness != signal.ExactnessScraped {
			t.Fatalf("%s = %#v", key, observation)
		}
		if observation.Object.Kind != "mysql.replication_channel" || observation.Object.ID != "default" {
			t.Fatalf("object=%#v", observation.Object)
		}
		return
	}
	t.Fatalf("missing %s", key)
}
