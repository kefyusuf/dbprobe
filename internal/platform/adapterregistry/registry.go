package adapterregistry

import (
	"fmt"

	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type Registry struct {
	adapters []adapter.Adapter
}

func New(adapters ...adapter.Adapter) (*Registry, error) {
	seen := make(map[string]struct{}, len(adapters))
	for _, candidate := range adapters {
		metadata := candidate.Metadata()
		if metadata.ID == "" {
			return nil, fmt.Errorf("adapter ID must not be empty")
		}
		if metadata.ContractVersion != adapter.ContractVersion {
			return nil, fmt.Errorf("adapter %q contract version %q does not match %q", metadata.ID, metadata.ContractVersion, adapter.ContractVersion)
		}
		if _, exists := seen[metadata.ID]; exists {
			return nil, fmt.Errorf("duplicate adapter ID %q", metadata.ID)
		}
		seen[metadata.ID] = struct{}{}
	}
	return &Registry{adapters: append([]adapter.Adapter(nil), adapters...)}, nil
}

func (r *Registry) Resolve(spec adapter.TargetSpec) (adapter.Adapter, error) {
	for _, candidate := range r.adapters {
		if candidate.Match(spec) {
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("no adapter matches target scheme %q", spec.Scheme)
}
