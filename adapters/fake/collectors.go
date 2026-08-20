package fake

import (
	"context"
	"sync"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

var instanceRef = object.Ref{Kind: "fake.instance", ID: "local"}

type healthCollector struct{}

func (healthCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "fake.health",
		Requires: []capability.Capability{"activity.sessions"},
		Produces: []signal.Key{"core.connections.used", "core.connections.limit"},
		Strategy: collector.StrategySnapshot,
	}
}

func (healthCollector) Collect(_ context.Context, req collector.Request) ([]signal.Observation, error) {
	return []signal.Observation{
		signal.NumberObservation("core.connections.used", instanceRef, 12, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt),
		signal.NumberObservation("core.connections.limit", instanceRef, 100, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, req.CollectedAt),
	}, nil
}

type workloadCollector struct {
	mu    sync.Mutex
	calls int
}

func (*workloadCollector) Descriptor() collector.Descriptor {
	return collector.Descriptor{
		ID:       "fake.workload",
		Requires: []capability.Capability{"workload.query_summary"},
		Produces: []signal.Key{"core.query.calls"},
		Strategy: collector.StrategyCounter,
	}
}

func (c *workloadCollector) Collect(_ context.Context, req collector.Request) ([]signal.Observation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	value := 100.0
	if c.calls >= 2 {
		value = 140
	}
	return []signal.Observation{
		signal.NumberObservation("core.query.calls", instanceRef, value, signal.UnitCount, signal.ExactnessCumulative, signal.SensitivityMetadata, req.CollectedAt),
	}, nil
}
