package temporal

import (
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestDeriveEventsConvertsChangesAndRegressions(t *testing.T) {
	at := time.Date(2026, 8, 21, 8, 0, 0, 0, time.UTC)
	ref := object.Ref{Kind: "mysql.query", ID: "shop:abc"}
	diff := Diff{Changes: []Change{
		{Kind: ChangeReset, Key: "counter", Object: object.Ref{Kind: "mysql.instance", ID: "db"}},
		{Kind: ChangeAdded, Key: "added", Object: object.Ref{Kind: "mysql.index", ID: "shop.orders.idx"}},
		{Kind: ChangeRemoved, Key: "removed", Object: object.Ref{Kind: "mysql.table", ID: "shop.old"}},
	}}
	regressions := []QueryRegression{{Object: ref, PreviousMeanLatencyMS: 10, CurrentMeanLatencyMS: 30, Ratio: 3, AddedLatencyMS: 600, CurrentCalls: 30}}
	got := DeriveEvents(diff, regressions, at)
	if len(got) != 4 {
		t.Fatalf("events=%#v", got)
	}
	assertEvent(t, got, EventSignalReset, "mysql.instance", "db", "counter")
	assertEvent(t, got, EventObjectAppeared, "mysql.index", "shop.orders.idx", "added")
	assertEvent(t, got, EventObjectDisappeared, "mysql.table", "shop.old", "removed")
	assertEvent(t, got, EventQueryRegression, "mysql.query", "shop:abc", "")
	for _, event := range got {
		if event.CollectedAt != at {
			t.Fatalf("event time=%v", event.CollectedAt)
		}
	}
}

func assertEvent(t *testing.T, events []Event, typ EventType, kind, id string, key signal.Key) {
	t.Helper()
	for _, event := range events {
		if event.Type == typ && event.Object.Kind == kind && event.Object.ID == id && event.SignalKey == key {
			return
		}
	}
	t.Fatalf("missing event type=%s object=%s:%s key=%s in %#v", typ, kind, id, key, events)
}
