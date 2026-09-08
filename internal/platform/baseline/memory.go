package baseline

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
)

type Memory struct {
	mu      sync.RWMutex
	targets map[string]map[string]temporal.Snapshot
}

func NewMemory() *Memory {
	return &Memory{targets: make(map[string]map[string]temporal.Snapshot)}
}

func (m *Memory) Save(_ context.Context, snapshot temporal.Snapshot) error {
	if snapshot.ID == "" || snapshot.TargetFingerprint == "" || snapshot.CollectedAt.IsZero() {
		return fmt.Errorf("invalid snapshot")
	}
	copy, err := cloneSnapshot(snapshot)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	byID := m.targets[snapshot.TargetFingerprint]
	if byID == nil {
		byID = make(map[string]temporal.Snapshot)
		m.targets[snapshot.TargetFingerprint] = byID
	}
	if _, exists := byID[snapshot.ID]; exists {
		return nil
	}
	byID[snapshot.ID] = copy
	return nil
}

func (m *Memory) Latest(_ context.Context, target string) (*temporal.Snapshot, error) {
	items, err := m.list(target)
	if err != nil {
		return nil, err
	}
	copy, err := cloneSnapshot(items[0])
	if err != nil {
		return nil, err
	}
	return &copy, nil
}

func (m *Memory) Previous(_ context.Context, target string, before time.Time) (*temporal.Snapshot, error) {
	items, err := m.list(target)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.CollectedAt.Before(before) {
			copy, err := cloneSnapshot(item)
			if err != nil {
				return nil, err
			}
			return &copy, nil
		}
	}
	return nil, temporal.ErrSnapshotNotFound
}

func (m *Memory) List(_ context.Context, target string, limit int) ([]temporal.Snapshot, error) {
	items, err := m.list(target)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}
	out := make([]temporal.Snapshot, 0, limit)
	for _, item := range items[:limit] {
		copy, err := cloneSnapshot(item)
		if err != nil {
			return nil, err
		}
		out = append(out, copy)
	}
	return out, nil
}

func (m *Memory) list(target string) ([]temporal.Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	byID := m.targets[target]
	if len(byID) == 0 {
		return nil, temporal.ErrSnapshotNotFound
	}
	out := make([]temporal.Snapshot, 0, len(byID))
	for _, item := range byID {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CollectedAt.Equal(out[j].CollectedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CollectedAt.After(out[j].CollectedAt)
	})
	return out, nil
}

func cloneSnapshot(snapshot temporal.Snapshot) (temporal.Snapshot, error) {
	return temporal.NewSnapshot(temporal.SnapshotInput{
		TargetFingerprint: snapshot.TargetFingerprint,
		Engine:            snapshot.Engine,
		AdapterID:         snapshot.AdapterID,
		AdapterVersion:    snapshot.AdapterVersion,
		CollectedAt:       snapshot.CollectedAt,
		Capabilities:      snapshot.Capabilities,
		Observations:      snapshot.Observations,
		Deltas:            snapshot.Deltas,
		Findings:          snapshot.Findings,
	})
}
