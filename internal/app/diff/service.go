package diff

import (
	"context"
	"fmt"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
)

const SchemaVersion = "dbprobe.diff/v1alpha1"

type Report struct {
	SchemaVersion       string                     `json:"schema_version"`
	TargetFingerprint   string                     `json:"target_fingerprint"`
	PreviousSnapshotID  string                     `json:"previous_snapshot_id"`
	CurrentSnapshotID   string                     `json:"current_snapshot_id"`
	PreviousCollectedAt time.Time                  `json:"previous_collected_at"`
	CurrentCollectedAt  time.Time                  `json:"current_collected_at"`
	Changes             []temporal.Change          `json:"changes"`
	QueryRegressions    []temporal.QueryRegression `json:"query_regressions"`
	Events              []temporal.Event           `json:"events"`
}

type Service struct {
	store temporal.Store
}

func New(store temporal.Store) *Service {
	return &Service{store: store}
}

func (s *Service) Run(ctx context.Context, targetFingerprint string, metrics *temporal.MetricPair) (Report, error) {
	if s == nil || s.store == nil {
		return Report{}, fmt.Errorf("diff store is required")
	}
	if targetFingerprint == "" {
		return Report{}, fmt.Errorf("target fingerprint is required")
	}
	current, err := s.store.Latest(ctx, targetFingerprint)
	if err != nil {
		return Report{}, err
	}
	previous, err := s.store.Previous(ctx, targetFingerprint, current.CollectedAt)
	if err != nil {
		return Report{}, err
	}
	diffResult, err := temporal.Compare(*previous, *current)
	if err != nil {
		return Report{}, err
	}
	regressions := []temporal.QueryRegression{}
	if metrics != nil {
		regressions = temporal.DetectQueryRegressions(*previous, *current, *metrics, temporal.DefaultQueryRegressionPolicy())
	}
	events := temporal.DeriveEvents(diffResult, regressions, current.CollectedAt)
	return Report{
		SchemaVersion:       SchemaVersion,
		TargetFingerprint:   targetFingerprint,
		PreviousSnapshotID:  previous.ID,
		CurrentSnapshotID:   current.ID,
		PreviousCollectedAt: previous.CollectedAt,
		CurrentCollectedAt:  current.CollectedAt,
		Changes:             diffResult.Changes,
		QueryRegressions:    regressions,
		Events:              events,
	}, nil
}
