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

type partialCounterCollector struct {
	warnPhase collector.Phase
}

func (partialCounterCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "partial.counter",
		Strategy: collector.StrategyCounter,
	}
}

func (c partialCounterCollector) Collect(_ context.Context, req collector.Request) ([]signal.Observation, error) {
	value := 10.0
	if req.Phase == collector.PhaseSampleB {
		value = 15
	}
	observations := []signal.Observation{
		signal.NumberObservation(
			"partial.counter.value",
			object.Ref{Kind: "test.object", ID: "stable"},
			value,
			signal.UnitCount,
			signal.ExactnessCumulative,
			signal.SensitivityMetadata,
			time.Unix(1, 0).UTC(),
		),
	}
	if req.Phase == c.warnPhase {
		return observations, errors.New("bounded counter scan truncated")
	}
	return observations, nil
}

func TestPlannerComputesOverlappingDeltaFromPartialCounterEvidence(t *testing.T) {
	for _, phase := range []collector.Phase{collector.PhaseSampleA, collector.PhaseSampleB} {
		t.Run(string(phase), func(t *testing.T) {
			planner := New(partialWarningWaiter{}, func() time.Time { return time.Unix(1, 0).UTC() })
			result, err := planner.Run(
				context.Background(),
				capability.New(),
				[]collector.Collector{partialCounterCollector{warnPhase: phase}},
				time.Second,
			)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Observations) != 1 {
				t.Fatalf("phase=%q observations=%#v", phase, result.Observations)
			}
			if len(result.Deltas) != 1 || result.Deltas[0].Delta != 5 || result.Deltas[0].RatePerSecond != 5 {
				t.Fatalf("phase=%q deltas=%#v", phase, result.Deltas)
			}
			if len(result.Warnings) != 1 || result.Warnings[0].CollectorID != "partial.counter" || result.Warnings[0].Reason != "bounded counter scan truncated" {
				t.Fatalf("phase=%q warnings=%#v", phase, result.Warnings)
			}
		})
	}
}
