package collectors

import (
	"context"
	"strings"
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestTransactionCollectorKeepsEphemeralIdentityOutOfStableObjects(t *testing.T) {
	q := &recordingQueryer{rows: [][]string{{"123", "77", "RUNNING", "600", "42", "10"}}}
	got, err := NewTransactions(q, 50).Collect(context.Background(), collector.Request{})
	if err != nil {
		t.Fatal(err)
	}
	assertOperationalNumber(t, got, "mysql.transaction.age_seconds", "mysql.transaction", "ephemeral:123", 600)
	assertOperationalNumber(t, got, "mysql.transaction.rows_locked", "mysql.transaction", "ephemeral:123", 42)
	assertOperationalText(t, got, "mysql.transaction.state", "RUNNING")
}

func TestLockCollectorUsesStableTableObjectAndOpaqueEdgeEvidence(t *testing.T) {
	q := &recordingQueryer{rows: [][]string{{"123", "122", "shop", "orders", "PRIMARY"}}}
	got, err := NewLocks(q, "instance-1", 100).Collect(context.Background(), collector.Request{})
	if err != nil {
		t.Fatal(err)
	}
	assertOperationalNumber(t, got, "mysql.lock_wait.count", "mysql.table", "shop.orders", 1)
	assertOperationalText(t, got, "mysql.lock_wait.edge", "123->122")
	assertOperationalText(t, got, "mysql.lock_wait.index", "PRIMARY")
}

func TestReplicationCollectorSeparatesStateAndNeverSelectsErrorMessageText(t *testing.T) {
	q := &recordingQueryer{rows: [][]string{
		{"receiver", "default", "ON", "0"},
		{"applier", "default", "OFF", "0"},
		{"worker:1", "default", "OFF", "1062"},
	}}
	got, err := NewReplication(q, 100).Collect(context.Background(), collector.Request{})
	if err != nil {
		t.Fatal(err)
	}
	assertOperationalBool(t, got, "mysql.replication.receiver_on", "mysql.replication_channel", "default", true)
	assertOperationalBool(t, got, "mysql.replication.applier_on", "mysql.replication_channel", "default", false)
	assertOperationalNumber(t, got, "mysql.replication.error_code", "mysql.replication_worker", "default:worker:1", 1062)
	assertOperationalText(t, got, "mysql.replication.receiver_state", "ON")
	assertOperationalText(t, got, "mysql.replication.applier_state", "OFF")
	if strings.Contains(strings.ToUpper(q.query), "LAST_ERROR_MESSAGE") {
		t.Fatalf("replication collector selects potentially sensitive error text: %s", q.query)
	}
}

func assertOperationalNumber(t *testing.T, observations []signal.Observation, key signal.Key, kind, id string, want float64) {
	t.Helper()
	for _, observation := range observations {
		if observation.Key == key && observation.Object.Kind == kind && observation.Object.ID == id {
			value, ok := observation.Numeric()
			if !ok || value != want {
				t.Fatalf("%s = %#v", key, observation)
			}
			return
		}
	}
	t.Fatalf("missing %s on %s:%s", key, kind, id)
}

func assertOperationalText(t *testing.T, observations []signal.Observation, key signal.Key, want string) {
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

func assertOperationalBool(t *testing.T, observations []signal.Observation, key signal.Key, kind, id string, want bool) {
	t.Helper()
	for _, observation := range observations {
		if observation.Key == key && observation.Object.Kind == kind && observation.Object.ID == id {
			if observation.Boolean == nil || *observation.Boolean != want {
				t.Fatalf("%s = %#v", key, observation.Boolean)
			}
			return
		}
	}
	t.Fatalf("missing %s", key)
}
