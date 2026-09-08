package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/app/inspect"
)

type fakeInspectRunner struct {
	calls  int
	target string
	window time.Duration
	report inspect.Report
	err    error
}

func (r *fakeInspectRunner) Run(_ context.Context, target string, window time.Duration) (inspect.Report, error) {
	r.calls++
	r.target = target
	r.window = window
	return r.report, r.err
}

func TestRunInspectWithRunnerRendersJSON(t *testing.T) {
	runner := &fakeInspectRunner{report: inspect.Report{SchemaVersion: inspect.SchemaVersion}}
	var out bytes.Buffer
	if err := runInspectWithRunner(context.Background(), &out, "fake://local", "json", time.Second, runner); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 || runner.target != "fake://local" || runner.window != time.Second {
		t.Fatalf("runner=%#v", runner)
	}
	if !strings.Contains(out.String(), `"schema_version": "dbprobe.inspect/v1alpha1"`) {
		t.Fatalf("json=%s", out.String())
	}
}

func TestRunInspectWithRunnerValidatesBeforeRunner(t *testing.T) {
	for _, tc := range []struct {
		target string
		format string
		window time.Duration
		want   string
	}{
		{target: "", format: "json", window: time.Second, want: "target"},
		{target: "fake://local", format: "xml", window: time.Second, want: "format"},
		{target: "fake://local", format: "json", window: 0, want: "sample window"},
	} {
		runner := &fakeInspectRunner{}
		err := runInspectWithRunner(context.Background(), &bytes.Buffer{}, tc.target, tc.format, tc.window, runner)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("err=%v", err)
		}
		if runner.calls != 0 {
			t.Fatalf("calls=%d", runner.calls)
		}
	}
}

func TestRunInspectWithRunnerPropagatesServiceError(t *testing.T) {
	runner := &fakeInspectRunner{err: errors.New("inspect failed")}
	err := runInspectWithRunner(context.Background(), &bytes.Buffer{}, "fake://local", "text", time.Second, runner)
	if err == nil || err.Error() != "inspect failed" {
		t.Fatalf("err=%v", err)
	}
}
