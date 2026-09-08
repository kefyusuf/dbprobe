package collector

import (
	"context"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type Strategy string
type Phase string

const (
	StrategySnapshot Strategy = "snapshot"
	StrategyCounter  Strategy = "counter"
	PhaseSingle      Phase    = "single"
	PhaseSampleA     Phase    = "sample_a"
	PhaseSampleB     Phase    = "sample_b"
)

type Descriptor struct {
	ID       string
	Requires []capability.Capability
	Produces []signal.Key
	Strategy Strategy
}

type Request struct {
	Phase       Phase
	CollectedAt time.Time
}

type Collector interface {
	Descriptor() Descriptor
	Collect(context.Context, Request) ([]signal.Observation, error)
}
