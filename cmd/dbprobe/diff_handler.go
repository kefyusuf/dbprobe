package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	appdiff "github.com/kefyusuf/dbprobe/internal/app/diff"
	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	jsonsurface "github.com/kefyusuf/dbprobe/internal/surfaces/json"
	"github.com/kefyusuf/dbprobe/internal/surfaces/terminal"
)

type diffTargetRunner interface {
	Run(context.Context, string) (appdiff.Report, error)
}

func runDiff(ctx context.Context, out io.Writer, target, format string, registry *adapterregistry.Registry, store temporal.Store) error {
	if registry == nil {
		return fmt.Errorf("adapter registry is required")
	}
	if store == nil {
		return fmt.Errorf("history store is required")
	}
	runner := appdiff.NewTargetService(registry, store, queryRegressionMetrics)
	return runDiffWithRunner(ctx, out, target, format, runner)
}

func runDiffWithRunner(ctx context.Context, out io.Writer, target, format string, runner diffTargetRunner) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("diff target is required")
	}
	if format != "text" && format != "json" {
		return fmt.Errorf("unsupported format %q: expected text or json", format)
	}
	if out == nil {
		return fmt.Errorf("diff output writer is required")
	}
	if runner == nil {
		return fmt.Errorf("diff runner is required")
	}
	report, err := runner.Run(ctx, target)
	if err != nil {
		return err
	}
	if format == "json" {
		return jsonsurface.RenderDiff(out, report)
	}
	return terminal.RenderDiff(out, report)
}
