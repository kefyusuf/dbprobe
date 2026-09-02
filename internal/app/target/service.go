package target

import (
	"context"
	"fmt"

	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type Service struct {
	registry *adapterregistry.Registry
}

func New(registry *adapterregistry.Registry) *Service {
	return &Service{registry: registry}
}

func (s *Service) Resolve(ctx context.Context, rawTarget string) (adapter.TargetMetadata, error) {
	if s == nil || s.registry == nil {
		return adapter.TargetMetadata{}, fmt.Errorf("target adapter registry is required")
	}
	spec, err := adapter.ParseTarget(rawTarget)
	if err != nil {
		return adapter.TargetMetadata{}, err
	}
	selected, err := s.registry.Resolve(spec)
	if err != nil {
		return adapter.TargetMetadata{}, err
	}
	runtime, err := selected.Open(ctx, spec, adapter.OpenOptions{})
	if err != nil {
		return adapter.TargetMetadata{}, err
	}
	target := runtime.Target()
	if err := runtime.Close(); err != nil {
		return adapter.TargetMetadata{}, fmt.Errorf("close target runtime: %w", err)
	}
	if target.Engine == "" || target.AdapterID == "" || target.Fingerprint == "" {
		return adapter.TargetMetadata{}, fmt.Errorf("adapter returned incomplete target metadata")
	}
	return target, nil
}
