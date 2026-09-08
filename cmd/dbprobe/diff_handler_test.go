package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appdiff "github.com/kefyusuf/dbprobe/internal/app/diff"
)

type fakeDiffTargetRunner struct {
	calls  int
	target string
	report appdiff.Report
	err    error
}

func (r *fakeDiffTargetRunner) Run(_ context.Context, target string) (appdiff.Report, error) {
	r.calls++
	r.target = target
	return r.report, r.err
}

func TestRunDiffWithRunnerRendersJSON(t *testing.T) {
	runner := &fakeDiffTargetRunner{report: appdiff.Report{
		SchemaVersion:       appdiff.SchemaVersion,
		TargetFingerprint:   "fp",
		PreviousCollectedAt: time.Unix(1, 0),
		CurrentCollectedAt:  time.Unix(2, 0),
	}}
	var out bytes.Buffer
	if err := runDiffWithRunner(context.Background(), &out, "mysql://local/shop", "json", runner); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 || runner.target != "mysql://local/shop" {
		t.Fatalf("runner=%#v", runner)
	}
	if !strings.Contains(out.String(), `"schema_version": "dbprobe.diff/v1alpha1"`) {
		t.Fatalf("json=%s", out.String())
	}
}

func TestRunDiffWithRunnerRendersText(t *testing.T) {
	runner := &fakeDiffTargetRunner{report: appdiff.Report{TargetFingerprint: "fp"}}
	var out bytes.Buffer
	if err := runDiffWithRunner(context.Background(), &out, "mysql://local/shop", "text", runner); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "dbprobe diff · fp") {
		t.Fatalf("text=%s", out.String())
	}
}

func TestRunDiffWithRunnerValidatesBeforeCallingRunner(t *testing.T) {
	for _, tc := range []struct {
		target string
		format string
		want   string
	}{
		{target: "", format: "json", want: "target"},
		{target: "mysql://local/shop", format: "xml", want: "format"},
	} {
		runner := &fakeDiffTargetRunner{}
		err := runDiffWithRunner(context.Background(), &bytes.Buffer{}, tc.target, tc.format, runner)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("err=%v", err)
		}
		if runner.calls != 0 {
			t.Fatalf("calls=%d", runner.calls)
		}
	}
}

func TestRunDiffWithRunnerPropagatesServiceError(t *testing.T) {
	runner := &fakeDiffTargetRunner{err: errors.New("history unavailable")}
	err := runDiffWithRunner(context.Background(), &bytes.Buffer{}, "mysql://local/shop", "json", runner)
	if err == nil || err.Error() != "history unavailable" {
		t.Fatalf("err=%v", err)
	}
}
