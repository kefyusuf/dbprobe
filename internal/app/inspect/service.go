package inspect

import (
	"context"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/collection"
	corefindings "github.com/kefyusuf/dbprobe/internal/core/findings"
	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

const SchemaVersion = "dbprobe.inspect/v1alpha1"

type Report struct {
	SchemaVersion string                  `json:"schema_version"`
	CollectedAt   time.Time               `json:"collected_at"`
	Target        adapter.TargetMetadata  `json:"target"`
	Capabilities  []capability.Capability `json:"capabilities"`
	Security      adapter.SecurityProfile `json:"security"`
	Observations  []signal.Observation    `json:"observations"`
	Deltas        []signal.Delta          `json:"deltas"`
	Findings      []finding.Finding       `json:"findings"`
	Warnings      []collection.Warning    `json:"warnings"`
}

type Service struct {
	registry *adapterregistry.Registry
	planner  *collection.Planner
	history  temporal.Store
}

func New(registry *adapterregistry.Registry, planner *collection.Planner) *Service {
	return &Service{registry: registry, planner: planner}
}

func (s *Service) WithHistory(store temporal.Store) *Service {
	if s == nil {
		return nil
	}
	clone := *s
	clone.history = store
	return &clone
}

func (s *Service) Run(ctx context.Context, rawTarget string, sampleWindow time.Duration) (Report, error) {
	spec, err := adapter.ParseTarget(rawTarget)
	if err != nil {
		return Report{}, err
	}
	selected, err := s.registry.Resolve(spec)
	if err != nil {
		return Report{}, err
	}
	adapterVersion := selected.Metadata().Version
	runtime, err := selected.Open(ctx, spec, adapter.OpenOptions{})
	if err != nil {
		return Report{}, err
	}
	defer runtime.Close()

	target := runtime.Target()
	caps := runtime.Capabilities()
	security := runtime.SecurityProfile()
	collected, err := s.planner.Run(ctx, caps, runtime.Collectors(), sampleWindow)
	if err != nil {
		return Report{}, err
	}

	findings := []finding.Finding{}
	analysis := finding.AnalysisContext{
		Capabilities: caps,
		Current:      collected.Observations,
		Previous:     []signal.Observation{},
		Deltas:       collected.Deltas,
	}
	rules := corefindings.Rules()
	rules = append(rules, runtime.Rules()...)
	for _, rule := range rules {
		if !caps.HasAll(rule.Requires()) {
			continue
		}
		findings = append(findings, rule.Evaluate(analysis)...)
	}

	deltas := collected.Deltas
	if deltas == nil {
		deltas = []signal.Delta{}
	}
	warnings := collected.Warnings
	if warnings == nil {
		warnings = []collection.Warning{}
	}
	observations := collected.Observations
	if observations == nil {
		observations = []signal.Observation{}
	}

	report := Report{
		SchemaVersion: SchemaVersion,
		CollectedAt:   time.Now().UTC(),
		Target:        target,
		Capabilities:  caps.List(),
		Security:      security,
		Observations:  observations,
		Deltas:        deltas,
		Findings:      findings,
		Warnings:      warnings,
	}
	if warning := persistHistory(ctx, s.history, report, adapterVersion); warning != nil {
		report.Warnings = append(report.Warnings, *warning)
	}
	return report, nil
}
