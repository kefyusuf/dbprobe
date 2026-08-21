package collection_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/collection"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type instantWaiter struct{ calls atomic.Int32 }

func (w *instantWaiter) Wait(context.Context, time.Duration) error {
	w.calls.Add(1)
	return nil
}

type concurrencyTracker struct {
	current atomic.Int32
	maximum atomic.Int32
}

func (t *concurrencyTracker) enter() {
	n := t.current.Add(1)
	for {
		m := t.maximum.Load()
		if n <= m || t.maximum.CompareAndSwap(m, n) {
			return
		}
	}
}

func (t *concurrencyTracker) leave() { t.current.Add(-1) }

type gateCollector struct {
	id       string
	strategy collector.Strategy
	tracker  *concurrencyTracker
	release  <-chan struct{}
	calls    atomic.Int32
}

func (c *gateCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{ID: c.id, Strategy: c.strategy}
}

func (c *gateCollector) Collect(ctx context.Context, req collector.Request) ([]signal.Observation, error) {
	c.calls.Add(1)
	c.tracker.enter()
	defer c.tracker.leave()
	select {
	case <-c.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	value := float64(c.calls.Load() * 10)
	return []signal.Observation{
		signal.NumberObservation(signal.Key(c.id), object.Ref{Kind: "test", ID: c.id}, value, signal.UnitCount, signal.ExactnessCumulative, signal.SensitivityMetadata, req.CollectedAt),
	}, nil
}

func TestPlannerDefaultsToFourConcurrentCollectors(t *testing.T) {
	tracker := &concurrencyTracker{}
	release := make(chan struct{})
	collectors := make([]collector.Collector, 0, 6)
	for i := 0; i < 6; i++ {
		collectors = append(collectors, &gateCollector{id: string(rune('a' + i)), strategy: collector.StrategySnapshot, tracker: tracker, release: release})
	}
	planner := collection.New(&instantWaiter{}, time.Now)
	done := make(chan error, 1)
	go func() {
		_, err := planner.Run(context.Background(), capability.New(), collectors, time.Second)
		done <- err
	}()

	deadline := time.After(time.Second)
	for tracker.maximum.Load() < 4 {
		select {
		case <-deadline:
			t.Fatal("did not reach default concurrency 4")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if got := tracker.maximum.Load(); got != 4 {
		t.Fatalf("default max concurrency = %d; want 4", got)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPlannerRunsSnapshotCollectorsWithConcurrencyBound(t *testing.T) {
	tracker := &concurrencyTracker{}
	release := make(chan struct{})
	collectors := make([]collector.Collector, 0, 6)
	for i := 0; i < 6; i++ {
		collectors = append(collectors, &gateCollector{id: string(rune('a' + i)), strategy: collector.StrategySnapshot, tracker: tracker, release: release})
	}

	planner := collection.New(&instantWaiter{}, time.Now).WithMaxConcurrency(2)
	done := make(chan struct{})
	var result collection.Result
	var err error
	go func() {
		result, err = planner.Run(context.Background(), capability.New(), collectors, time.Second)
		close(done)
	}()

	deadline := time.After(time.Second)
	for tracker.maximum.Load() < 2 {
		select {
		case <-deadline:
			t.Fatal("did not reach concurrency 2")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if got := tracker.maximum.Load(); got != 2 {
		t.Fatalf("max concurrency = %d; want 2", got)
	}
	close(release)
	<-done
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Observations) != 6 {
		t.Fatalf("observations = %d; want 6", len(result.Observations))
	}
}

type phaseCollector struct {
	id      string
	tracker *concurrencyTracker
	mu      sync.Mutex
	phases  []collector.Phase
}

func (c *phaseCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{ID: c.id, Strategy: collector.StrategyCounter}
}

func (c *phaseCollector) Collect(_ context.Context, req collector.Request) ([]signal.Observation, error) {
	c.tracker.enter()
	defer c.tracker.leave()
	c.mu.Lock()
	c.phases = append(c.phases, req.Phase)
	call := len(c.phases)
	c.mu.Unlock()
	value := 100.0
	if call == 2 {
		value = 140
	}
	return []signal.Observation{
		signal.NumberObservation(signal.Key(c.id), object.Ref{Kind: "test", ID: c.id}, value, signal.UnitCount, signal.ExactnessCumulative, signal.SensitivityMetadata, req.CollectedAt),
	}, nil
}

type checkingWaiter struct {
	calls    atomic.Int32
	counters []*phaseCollector
	t        *testing.T
}

func (w *checkingWaiter) Wait(context.Context, time.Duration) error {
	w.calls.Add(1)
	for _, counter := range w.counters {
		counter.mu.Lock()
		phaseCount := len(counter.phases)
		counter.mu.Unlock()
		if phaseCount != 1 {
			w.t.Errorf("wait observed %s phase count = %d; want 1", counter.id, phaseCount)
		}
	}
	return nil
}

func TestPlannerRunsCounterPhasesWithConcurrencyBoundAndSingleWait(t *testing.T) {
	tracker := &concurrencyTracker{}
	counters := make([]*phaseCollector, 4)
	collectors := make([]collector.Collector, 4)
	for i := range counters {
		counters[i] = &phaseCollector{id: string(rune('a' + i)), tracker: tracker}
		collectors[i] = counters[i]
	}
	waiter := &checkingWaiter{counters: counters, t: t}
	planner := collection.New(waiter, time.Now).WithMaxConcurrency(2)

	result, err := planner.Run(context.Background(), capability.New(), collectors, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if waiter.calls.Load() != 1 {
		t.Fatalf("wait calls = %d; want 1", waiter.calls.Load())
	}
	if tracker.maximum.Load() > 2 {
		t.Fatalf("max concurrency = %d; want <=2", tracker.maximum.Load())
	}
	if len(result.Deltas) != 4 {
		t.Fatalf("deltas = %d; want 4", len(result.Deltas))
	}
	for _, counter := range counters {
		counter.mu.Lock()
		phases := append([]collector.Phase(nil), counter.phases...)
		counter.mu.Unlock()
		if len(phases) != 2 || phases[0] != collector.PhaseSampleA || phases[1] != collector.PhaseSampleB {
			t.Fatalf("%s phases = %v", counter.id, phases)
		}
	}
}

func TestPlannerCancellationStopsQueuedCollectors(t *testing.T) {
	tracker := &concurrencyTracker{}
	release := make(chan struct{})
	first := &gateCollector{id: "first", strategy: collector.StrategySnapshot, tracker: tracker, release: release}
	second := &gateCollector{id: "second", strategy: collector.StrategySnapshot, tracker: tracker, release: release}
	third := &gateCollector{id: "third", strategy: collector.StrategySnapshot, tracker: tracker, release: release}

	ctx, cancel := context.WithCancel(context.Background())
	planner := collection.New(&instantWaiter{}, time.Now).WithMaxConcurrency(1)
	done := make(chan error, 1)
	go func() {
		_, err := planner.Run(ctx, capability.New(), []collector.Collector{first, second, third}, time.Second)
		done <- err
	}()

	deadline := time.After(time.Second)
	for first.calls.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("first collector did not start")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("error = %v; want context.Canceled", err)
	}
	if second.calls.Load()+third.calls.Load() != 0 {
		t.Fatalf("queued collectors ran: second=%d third=%d", second.calls.Load(), third.calls.Load())
	}
}
