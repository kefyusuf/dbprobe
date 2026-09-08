package diff

import (
	"context"
	"errors"
	"testing"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type diffTargetResolver struct {
	target adapter.TargetMetadata
	err    error
	raw    string
}

func (r *diffTargetResolver) Resolve(_ context.Context, raw string) (adapter.TargetMetadata, error) {
	r.raw = raw
	return r.target, r.err
}

type diffReportRunner struct {
	fingerprint string
	metrics     *temporal.MetricPair
	err         error
}

func (r *diffReportRunner) Run(_ context.Context, fingerprint string, metrics *temporal.MetricPair) (Report, error) {
	r.fingerprint = fingerprint
	r.metrics = metrics
	if r.err != nil {
		return Report{}, r.err
	}
	return Report{TargetFingerprint: fingerprint}, nil
}

func TestTargetServiceResolvesFingerprintAndMetrics(t *testing.T) {
	resolver := &diffTargetResolver{target: adapter.TargetMetadata{Engine: "mysql", AdapterID: "mysql", Fingerprint: "fp"}}
	runner := &diffReportRunner{}
	service := newTargetService(resolver, runner, func(target adapter.TargetMetadata) *temporal.MetricPair {
		if target.Engine != "mysql" {
			t.Fatalf("target=%#v", target)
		}
		return &temporal.MetricPair{CallsKey: "core.query.calls", TotalLatencyKey: "mysql.query.total_latency_ms"}
	})
	report, err := service.Run(context.Background(), "mysql://user:secret@local/shop")
	if err != nil {
		t.Fatal(err)
	}
	if resolver.raw != "mysql://user:secret@local/shop" || runner.fingerprint != "fp" || runner.metrics == nil || runner.metrics.TotalLatencyKey != "mysql.query.total_latency_ms" || report.TargetFingerprint != "fp" {
		t.Fatalf("resolver=%#v runner=%#v report=%#v", resolver, runner, report)
	}
}

func TestTargetServiceStopsWhenTargetResolutionFails(t *testing.T) {
	resolver := &diffTargetResolver{err: errors.New("resolve failed")}
	runner := &diffReportRunner{}
	service := newTargetService(resolver, runner, nil)
	if _, err := service.Run(context.Background(), "bad"); err == nil {
		t.Fatal("expected error")
	}
	if runner.fingerprint != "" {
		t.Fatalf("diff runner called with %q", runner.fingerprint)
	}
}

func TestTargetServiceSupportsNoRegressionMetricProfile(t *testing.T) {
	resolver := &diffTargetResolver{target: adapter.TargetMetadata{Engine: "unknown", AdapterID: "x", Fingerprint: "fp"}}
	runner := &diffReportRunner{}
	service := newTargetService(resolver, runner, func(adapter.TargetMetadata) *temporal.MetricPair { return nil })
	if _, err := service.Run(context.Background(), "x://local"); err != nil {
		t.Fatal(err)
	}
	if runner.metrics != nil {
		t.Fatalf("metrics=%#v", runner.metrics)
	}
}
