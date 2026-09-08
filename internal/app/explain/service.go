package explain

import (
	"context"
	"fmt"
	"strings"

	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
)

const SchemaVersion = "dbprobe.explain/v1alpha1"

type Report struct {
	SchemaVersion string                 `json:"schema_version"`
	Target        adapter.TargetMetadata `json:"target"`
	Format        string                 `json:"format"`
	Estimated     bool                   `json:"estimated"`
	Sanitized     bool                   `json:"sanitized"`
	Plan          string                 `json:"plan"`
}

type Service struct{ registry *adapterregistry.Registry }

func New(registry *adapterregistry.Registry) *Service { return &Service{registry: registry} }

func (s *Service) Run(ctx context.Context, rawTarget, statement string) (Report, error) {
	if s == nil || s.registry == nil {
		return Report{}, fmt.Errorf("explain adapter registry is required")
	}
	spec, err := adapter.ParseTarget(rawTarget)
	if err != nil {
		return Report{}, err
	}
	selected, err := s.registry.Resolve(spec)
	if err != nil {
		return Report{}, err
	}
	runtime, err := selected.Open(ctx, spec, adapter.OpenOptions{})
	if err != nil {
		return Report{}, err
	}
	defer runtime.Close()

	if !runtime.Capabilities().Has(capability.Capability("query.explain")) {
		return Report{}, fmt.Errorf("target does not expose query.explain capability")
	}
	explainer, ok := runtime.(adapter.PlanExplainer)
	if !ok {
		return Report{}, fmt.Errorf("adapter violates query.explain contract")
	}
	result, err := explainer.ExplainPlan(ctx, adapter.ExplainRequest{Statement: statement})
	if err != nil {
		return Report{}, err
	}
	if result.Engine == "" || result.Engine != runtime.Target().Engine {
		return Report{}, fmt.Errorf("adapter returned an invalid explain engine")
	}
	if strings.TrimSpace(result.Format) == "" || strings.TrimSpace(result.Plan) == "" || !result.Estimated || !result.Sanitized {
		return Report{}, fmt.Errorf("adapter returned an invalid safe plan-only explain result")
	}
	return Report{
		SchemaVersion: SchemaVersion,
		Target:        runtime.Target(),
		Format:        result.Format,
		Estimated:     result.Estimated,
		Sanitized:     result.Sanitized,
		Plan:          result.Plan,
	}, nil
}
