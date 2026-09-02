package collection_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/collection"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type noopWaiter struct{}

func (noopWaiter) Wait(context.Context, time.Duration) error { return nil }

type testCollector struct {
	desc  collector.Descriptor
	calls int
	fn    func(call int, req collector.Request) ([]signal.Observation, error)
}

func (c *testCollector) Descriptor() collector.Descriptor { return c.desc }
func (c *testCollector) Collect(_ context.Context, req collector.Request) ([]signal.Observation, error) {
	c.calls++
	return c.fn(c.calls, req)
}

func number(key signal.Key, value float64, exactness signal.Exactness, at time.Time) signal.Observation {
	return signal.NumberObservation(key, object.Ref{Kind: "fake.instance", ID: "local"}, value, signal.UnitCount, exactness, signal.SensitivityMetadata, at)
}

func TestPlannerSkipsCollectorMissingCapability(t *testing.T) {
	c := &testCollector{
		desc: collector.Descriptor{ID: "needs.replication", Requires: []capability.Capability{"replication.status"}, Produces: []signal.Key{"core.replication.lag_seconds"}, Strategy: collector.StrategySnapshot},
		fn:   func(int, collector.Request) ([]signal.Observation, error) { return nil, nil },
	}
	p := collection.New(noopWaiter{}, time.Now)
	result, err := p.Run(context.Background(), capability.New(), []collector.Collector{c}, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if c.calls != 0 {
		t.Fatalf("collector called %d times; want 0", c.calls)
	}
	if len(result.Observations) != 0 || len(result.Warnings) != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestPlannerPreservesSuccessfulEvidenceWhenCollectorFails(t *testing.T) {
	good := &testCollector{
		desc: collector.Descriptor{ID: "good", Strategy: collector.StrategySnapshot},
		fn: func(_ int, req collector.Request) ([]signal.Observation, error) {
			return []signal.Observation{number("core.connections.used", 12, signal.ExactnessScraped, req.CollectedAt)}, nil
		},
	}
	bad := &testCollector{
		desc: collector.Descriptor{ID: "bad", Strategy: collector.StrategySnapshot},
		fn:   func(int, collector.Request) ([]signal.Observation, error) { return nil, errors.New("boom") },
	}
	p := collection.New(noopWaiter{}, time.Now)
	result, err := p.Run(context.Background(), capability.New(), []collector.Collector{bad, good}, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Observations) != 1 {
		t.Fatalf("observations = %d; want 1", len(result.Observations))
	}
	if len(result.Warnings) != 1 || result.Warnings[0].CollectorID != "bad" {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
}

func TestPlannerSamplesCounterTwiceAndComputesRate(t *testing.T) {
	c := &testCollector{
		desc: collector.Descriptor{ID: "counter", Produces: []signal.Key{"core.query.calls"}, Strategy: collector.StrategyCounter},
		fn: func(call int, req collector.Request) ([]signal.Observation, error) {
			value := 100.0
			if call == 2 {
				value = 140
			}
			return []signal.Observation{number("core.query.calls", value, signal.ExactnessCumulative, req.CollectedAt)}, nil
		},
	}
	p := collection.New(noopWaiter{}, time.Now)
	result, err := p.Run(context.Background(), capability.New(), []collector.Collector{c}, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if c.calls != 2 {
		t.Fatalf("calls = %d; want 2", c.calls)
	}
	if len(result.Observations) != 1 {
		t.Fatalf("observations = %d; want current B only", len(result.Observations))
	}
	if got, _ := result.Observations[0].Numeric(); got != 140 {
		t.Fatalf("current value = %v; want 140", got)
	}
	if len(result.Deltas) != 1 {
		t.Fatalf("deltas = %#v", result.Deltas)
	}
	if result.Deltas[0].Delta != 40 || result.Deltas[0].RatePerSecond != 4 {
		t.Fatalf("delta = %#v; want delta=40 rate=4", result.Deltas[0])
	}
}

func TestPlannerTreatsCounterDecreaseAsReset(t *testing.T) {
	c := &testCollector{
		desc: collector.Descriptor{ID: "counter", Produces: []signal.Key{"core.query.calls"}, Strategy: collector.StrategyCounter},
		fn: func(call int, req collector.Request) ([]signal.Observation, error) {
			value := 100.0
			if call == 2 {
				value = 5
			}
			return []signal.Observation{number("core.query.calls", value, signal.ExactnessCumulative, req.CollectedAt)}, nil
		},
	}
	p := collection.New(noopWaiter{}, time.Now)
	result, err := p.Run(context.Background(), capability.New(), []collector.Collector{c}, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Deltas) != 0 {
		t.Fatalf("deltas = %#v; want none after reset", result.Deltas)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].CollectorID != "counter" || !strings.Contains(result.Warnings[0].Reason, "reset") {
		t.Fatalf("warnings = %#v; want reset warning", result.Warnings)
	}
}
