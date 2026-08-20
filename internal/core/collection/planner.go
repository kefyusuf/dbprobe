package collection

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type Warning struct {
	CollectorID string `json:"collector_id"`
	Reason      string `json:"reason"`
}

type Result struct {
	Observations []signal.Observation `json:"observations"`
	Deltas       []signal.Delta       `json:"deltas,omitempty"`
	Warnings     []Warning            `json:"warnings,omitempty"`
}

type Waiter interface {
	Wait(context.Context, time.Duration) error
}

type RealWaiter struct{}

func (RealWaiter) Wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type Planner struct {
	waiter Waiter
	now    func() time.Time
}

func New(waiter Waiter, now func() time.Time) *Planner {
	return &Planner{waiter: waiter, now: now}
}

func (p *Planner) Run(ctx context.Context, caps capability.Set, collectors []collector.Collector, sampleWindow time.Duration) (Result, error) {
	result := Result{Observations: []signal.Observation{}, Deltas: []signal.Delta{}, Warnings: []Warning{}}
	var counters []collector.Collector
	firstSamples := make(map[string]signal.Observation)

	for _, c := range collectors {
		desc := c.Descriptor()
		if !caps.HasAll(desc.Requires) {
			continue
		}
		switch desc.Strategy {
		case collector.StrategySnapshot:
			obs, err := c.Collect(ctx, collector.Request{Phase: collector.PhaseSingle, CollectedAt: p.now()})
			if err != nil {
				result.Warnings = append(result.Warnings, Warning{CollectorID: desc.ID, Reason: err.Error()})
				continue
			}
			result.Observations = append(result.Observations, obs...)
		case collector.StrategyCounter:
			counters = append(counters, c)
			obs, err := c.Collect(ctx, collector.Request{Phase: collector.PhaseSampleA, CollectedAt: p.now()})
			if err != nil {
				result.Warnings = append(result.Warnings, Warning{CollectorID: desc.ID, Reason: err.Error()})
				continue
			}
			for _, o := range obs {
				firstSamples[desc.ID+"|"+identity(o)] = o
			}
		default:
			result.Warnings = append(result.Warnings, Warning{CollectorID: desc.ID, Reason: "unsupported collection strategy"})
		}
	}

	if len(counters) > 0 {
		if sampleWindow <= 0 {
			return Result{}, fmt.Errorf("sample window must be positive for counter collectors")
		}
		if err := p.waiter.Wait(ctx, sampleWindow); err != nil {
			return Result{}, err
		}
	}

	for _, c := range counters {
		desc := c.Descriptor()
		obs, err := c.Collect(ctx, collector.Request{Phase: collector.PhaseSampleB, CollectedAt: p.now()})
		if err != nil {
			result.Warnings = append(result.Warnings, Warning{CollectorID: desc.ID, Reason: err.Error()})
			continue
		}
		result.Observations = append(result.Observations, obs...)
		for _, current := range obs {
			previous, ok := firstSamples[desc.ID+"|"+identity(current)]
			if !ok {
				continue
			}
			a, aOK := previous.Numeric()
			b, bOK := current.Numeric()
			if !aOK || !bOK {
				continue
			}
			if b < a {
				result.Warnings = append(result.Warnings, Warning{CollectorID: desc.ID, Reason: fmt.Sprintf("counter reset detected for %s", identity(current))})
				continue
			}
			delta := b - a
			result.Deltas = append(result.Deltas, signal.Delta{Key: current.Key, Object: current.Object, Unit: current.Unit, Delta: delta, RatePerSecond: delta / sampleWindow.Seconds(), WindowSeconds: sampleWindow.Seconds(), Exactness: signal.ExactnessSampled})
		}
	}

	sort.Slice(result.Observations, func(i, j int) bool { return identity(result.Observations[i]) < identity(result.Observations[j]) })
	sort.Slice(result.Deltas, func(i, j int) bool {
		left := fmt.Sprintf("%s|%s|%s", result.Deltas[i].Key, result.Deltas[i].Object.Kind, result.Deltas[i].Object.ID)
		right := fmt.Sprintf("%s|%s|%s", result.Deltas[j].Key, result.Deltas[j].Object.Kind, result.Deltas[j].Object.ID)
		return left < right
	})
	sort.Slice(result.Warnings, func(i, j int) bool {
		if result.Warnings[i].CollectorID == result.Warnings[j].CollectorID {
			return result.Warnings[i].Reason < result.Warnings[j].Reason
		}
		return result.Warnings[i].CollectorID < result.Warnings[j].CollectorID
	})
	return result, nil
}

func identity(o signal.Observation) string {
	return fmt.Sprintf("%s|%s|%s", o.Key, o.Object.Kind, o.Object.ID)
}
