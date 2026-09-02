package collection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type partialWarningWaiter struct{}

func (partialWarningWaiter) Wait(context.Context, time.Duration) error { return nil }

type partialWarningCollector struct{}

func (partialWarningCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "partial.warning",
		Strategy: collector.StrategySnapshot,
	}
}

func (partialWarningCollector) Collect(context.Context, collector.Request) ([]signal.Observation, error) {
	return []signal.Observation{
		signal.NumberObservation(
			"partial.value",
			object.Ref{Kind: "test.object", ID: "stable"},
			1,
			signal.UnitCount,
			signal.ExactnessScraped,
			signal.SensitivityMetadata,
			time.Unix(1, 0).UTC(),
		),
	}, errors.New("bounded scan truncated")
}

func TestPlannerPreservesPartialEvidenceWhenCollectorWarns(t *testing.T) {
	planner := New(partialWarningWaiter{}, func() time.Time { return time.Unix(1, 0).UTC() })
	result, err := planner.Run(context.Background(), capability.New(), []collector.Collector{partialWarningCollector{}}, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Observations) != 1 || result.Observations[0].Key != "partial.value" {
		t.Fatalf("observations=%#v", result.Observations)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].CollectorID != "partial.warning" || result.Warnings[0].Reason != "bounded scan truncated" {
		t.Fatalf("warnings=%#v", result.Warnings)
	}
}
