package temporal

import (
	"fmt"
	"sort"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type EventType string

const (
	EventSignalReset       EventType = "signal_reset"
	EventObjectAppeared    EventType = "object_appeared"
	EventObjectDisappeared EventType = "object_disappeared"
	EventQueryRegression   EventType = "query_regression"
)

type Event struct {
	Type           EventType  `json:"type"`
	Object         object.Ref `json:"object"`
	SignalKey      signal.Key `json:"signal_key,omitempty"`
	Summary        string     `json:"summary"`
	Confidence     float64    `json:"confidence"`
	ObservedAfter  time.Time  `json:"observed_after"`
	ObservedBefore time.Time  `json:"observed_before"`
}

func DeriveEvents(diff Diff, regressions []QueryRegression, observedAfter, observedBefore time.Time) []Event {
	if observedAfter.IsZero() || observedBefore.IsZero() || observedBefore.Before(observedAfter) {
		return []Event{}
	}
	after := observedAfter.UTC()
	before := observedBefore.UTC()
	out := make([]Event, 0, len(diff.Changes)+len(regressions))
	for _, change := range diff.Changes {
		var typ EventType
		var summary string
		switch change.Kind {
		case ChangeReset:
			typ = EventSignalReset
			summary = fmt.Sprintf("Signal %s reset for %s:%s.", change.Key, change.Object.Kind, change.Object.ID)
		case ChangeAdded:
			typ = EventObjectAppeared
			summary = fmt.Sprintf("Observed %s:%s for signal %s.", change.Object.Kind, change.Object.ID, change.Key)
		case ChangeRemoved:
			typ = EventObjectDisappeared
			summary = fmt.Sprintf("No longer observed %s:%s for signal %s.", change.Object.Kind, change.Object.ID, change.Key)
		default:
			continue
		}
		out = append(out, Event{
			Type:           typ,
			Object:         change.Object,
			SignalKey:      change.Key,
			Summary:        summary,
			Confidence:     0.75,
			ObservedAfter:  after,
			ObservedBefore: before,
		})
	}
	for _, regression := range regressions {
		out = append(out, Event{
			Type:           EventQueryRegression,
			Object:         regression.Object,
			Summary:        fmt.Sprintf("Mean query latency increased from %.2fms to %.2fms (%.2fx) in the sampled window.", regression.PreviousMeanLatencyMS, regression.CurrentMeanLatencyMS, regression.Ratio),
			Confidence:     0.90,
			ObservedAfter:  after,
			ObservedBefore: before,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if objectIdentity(out[i].Object) != objectIdentity(out[j].Object) {
			return objectIdentity(out[i].Object) < objectIdentity(out[j].Object)
		}
		return out[i].SignalKey < out[j].SignalKey
	})
	return out
}
