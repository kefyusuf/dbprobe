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

func runInspect(ctx context.Context, out io.Writer, target, format string, sampleWindow time.Duration, registry *adapterregistry.Registry, store temporal.Store) error {
	if registry == nil {
		return fmt.Errorf("adapter registry is required")
	}
	planner := collection.New(collection.RealWaiter{}, time.Now)
	service := inspect.New(registry, planner)
	if store != nil {
		service = service.WithHistory(store)
	}
	return runInspectWithRunner(ctx, out, target, format, sampleWindow, service)
}

func runInspectWithRunner(ctx context.Context, out io.Writer, target, format string, sampleWindow time.Duration, runner inspectRunner) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("inspect target is required")
	}
	if format != "text" && format != "json" {
		return fmt.Errorf("unsupported format %q: expected text or json", format)
	}
	if sampleWindow <= 0 {
		return fmt.Errorf("sample window must be positive")
	}
	if out == nil {
		return fmt.Errorf("inspect output writer is required")
	}
	if runner == nil {
		return fmt.Errorf("inspect runner is required")
	}
	report, err := runner.Run(ctx, target, sampleWindow)
	if err != nil {
		return err
	}
	if format == "json" {
		return jsonsurface.Render(out, report)
	}
	return terminal.Render(out, report)
}
