package temporal

import (
	"fmt"
	"sort"

	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type ChangeKind string

const (
	ChangeAdded   ChangeKind = "added"
	ChangeRemoved ChangeKind = "removed"
	ChangeChanged ChangeKind = "changed"
	ChangeReset   ChangeKind = "reset"
)

type Change struct {
	Kind         ChangeKind          `json:"kind"`
	Key          signal.Key          `json:"key"`
	Object       object.Ref          `json:"object"`
	Before       *signal.Observation `json:"before,omitempty"`
	After        *signal.Observation `json:"after,omitempty"`
	NumericDelta *float64            `json:"numeric_delta,omitempty"`
}

type Diff struct {
	PreviousSnapshotID string   `json:"previous_snapshot_id"`
	CurrentSnapshotID  string   `json:"current_snapshot_id"`
	Changes            []Change `json:"changes"`
}

func Compare(previous, current Snapshot) (Diff, error) {
	if previous.TargetFingerprint == "" || current.TargetFingerprint == "" || previous.TargetFingerprint != current.TargetFingerprint {
		return Diff{}, fmt.Errorf("cannot compare snapshots from different targets")
	}

	prev, prevAmbiguous := observationMap(previous.Observations)
	cur, curAmbiguous := observationMap(current.Observations)
	keys := make(map[string]struct{}, len(prev)+len(cur))
	for key := range prev {
		keys[key] = struct{}{}
	}
	for key := range cur {
		keys[key] = struct{}{}
	}

	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)

	changes := make([]Change, 0)
	for _, key := range ordered {
		if prevAmbiguous[key] || curAmbiguous[key] {
			continue
		}
		before, hadBefore := prev[key]
		after, hasAfter := cur[key]
		switch {
		case !hadBefore && hasAfter:
			a := cloneObservation(after)
			changes = append(changes, Change{Kind: ChangeAdded, Key: after.Key, Object: after.Object, After: &a})
		case hadBefore && !hasAfter:
			b := cloneObservation(before)
			changes = append(changes, Change{Kind: ChangeRemoved, Key: before.Key, Object: before.Object, Before: &b})
		case hadBefore && hasAfter:
			if observationEquivalent(before, after) {
				continue
			}
			b, a := cloneObservation(before), cloneObservation(after)
			change := Change{Kind: ChangeChanged, Key: after.Key, Object: after.Object, Before: &b, After: &a}
			if before.Number != nil && after.Number != nil {
				if before.Exactness == signal.ExactnessCumulative && after.Exactness == signal.ExactnessCumulative && *after.Number < *before.Number {
					change.Kind = ChangeReset
				} else {
					delta := *after.Number - *before.Number
					change.NumericDelta = &delta
				}
			}
			changes = append(changes, change)
		}
	}

	return Diff{PreviousSnapshotID: previous.ID, CurrentSnapshotID: current.ID, Changes: changes}, nil
}

func observationMap(values []signal.Observation) (map[string]signal.Observation, map[string]bool) {
	out := make(map[string]signal.Observation, len(values))
	ambiguous := make(map[string]bool)
	for _, value := range values {
		identity := observationIdentity(value)
		if _, exists := out[identity]; exists {
			ambiguous[identity] = true
		}
		out[identity] = value
	}
	return out, ambiguous
}

func observationIdentity(value signal.Observation) string {
	return string(value.Key) + "|" + value.Object.Kind + "|" + value.Object.ID
}

func observationEquivalent(a, b signal.Observation) bool {
	if a.Key != b.Key || a.Object != b.Object || a.Unit != b.Unit || a.Exactness != b.Exactness || a.Sensitivity != b.Sensitivity || a.Source != b.Source || a.Reason != b.Reason {
		return false
	}
	return equalFloatPtr(a.Number, b.Number) && equalStringPtr(a.Text, b.Text) && equalBoolPtr(a.Boolean, b.Boolean)
}

func equalFloatPtr(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func equalStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func equalBoolPtr(a, b *bool) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
