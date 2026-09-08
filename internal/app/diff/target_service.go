package diff

import (
	"context"

	targetapp "github.com/kefyusuf/dbprobe/internal/app/target"
	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type MetricResolver func(adapter.TargetMetadata) *temporal.MetricPair

type targetResolver interface {
	Resolve(context.Context, string) (adapter.TargetMetadata, error)
}

type reportRunner interface {
	Run(context.Context, string, *temporal.MetricPair) (Report, error)
}

type TargetService struct {
	targets targetResolver
	reports reportRunner
	metrics MetricResolver
}

func NewTargetService(registry *adapterregistry.Registry, store temporal.Store, metrics MetricResolver) *TargetService {
	return newTargetService(targetapp.New(registry), New(store), metrics)
}

func newTargetService(targets targetResolver, reports reportRunner, metrics MetricResolver) *TargetService {
	return &TargetService{targets: targets, reports: reports, metrics: metrics}
}

func (s *TargetService) Run(ctx context.Context, rawTarget string) (Report, error) {
	target, err := s.targets.Resolve(ctx, rawTarget)
	if err != nil {
		return Report{}, err
	}
	var pair *temporal.MetricPair
	if s.metrics != nil {
		pair = s.metrics(target)
	}
	return s.reports.Run(ctx, target.Fingerprint, pair)
}
