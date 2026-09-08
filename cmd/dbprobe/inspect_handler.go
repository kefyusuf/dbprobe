package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kefyusuf/dbprobe/internal/app/inspect"
	"github.com/kefyusuf/dbprobe/internal/core/collection"
	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	jsonsurface "github.com/kefyusuf/dbprobe/internal/surfaces/json"
	"github.com/kefyusuf/dbprobe/internal/surfaces/terminal"
)

type inspectRunner interface {
	Run(context.Context, string, time.Duration) (inspect.Report, error)
}

func runInspect(ctx context.Context, out io.Writer, target, format string, sampleWindow time.Duration, registry *adapterregistry.Registry, store temporal.Store, additionalWarnings ...collection.Warning) error {
	runner, err := newInspectRunner(registry, store)
	if err != nil {
		return err
	}
	return runInspectWithRunner(ctx, out, target, format, sampleWindow, runner, additionalWarnings...)
}

func newInspectRunner(registry *adapterregistry.Registry, store temporal.Store) (inspectRunner, error) {
	if registry == nil {
		return nil, fmt.Errorf("adapter registry is required")
	}
	planner := collection.New(collection.RealWaiter{}, time.Now)
	service := inspect.New(registry, planner)
	if store != nil {
		service = service.WithHistory(store)
	}
	return service, nil
}

func runInspectWithRunner(ctx context.Context, out io.Writer, target, format string, sampleWindow time.Duration, runner inspectRunner, additionalWarnings ...collection.Warning) error {
	report, err := collectInspectReport(ctx, out, target, format, sampleWindow, runner)
	if err != nil {
		return err
	}
	return renderInspectReport(out, format, report, additionalWarnings...)
}

func collectInspectReport(ctx context.Context, out io.Writer, target, format string, sampleWindow time.Duration, runner inspectRunner) (inspect.Report, error) {
	if strings.TrimSpace(target) == "" {
		return inspect.Report{}, fmt.Errorf("inspect target is required")
	}
	if format != "text" && format != "json" {
		return inspect.Report{}, fmt.Errorf("unsupported format %q: expected text or json", format)
	}
	if sampleWindow <= 0 {
		return inspect.Report{}, fmt.Errorf("sample window must be positive")
	}
	if out == nil {
		return inspect.Report{}, fmt.Errorf("inspect output writer is required")
	}
	if runner == nil {
		return inspect.Report{}, fmt.Errorf("inspect runner is required")
	}
	return runner.Run(ctx, target, sampleWindow)
}

func renderInspectReport(out io.Writer, format string, report inspect.Report, additionalWarnings ...collection.Warning) error {
	if len(additionalWarnings) > 0 {
		report.Warnings = append(report.Warnings, additionalWarnings...)
	}
	if format == "json" {
		return jsonsurface.Render(out, report)
	}
	return terminal.Render(out, report)
}
