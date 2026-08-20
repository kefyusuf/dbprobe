package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestInspectCommandRendersFakeJSON(t *testing.T) {
	cmd := newRootCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"inspect", "fake://local", "--format=json", "--sample-window=1ms"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr=%q", err, stderr.String())
	}

	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if report["schema_version"] != "dbprobe.inspect/v1alpha1" {
		t.Fatalf("schema_version = %#v", report["schema_version"])
	}
	target, ok := report["target"].(map[string]any)
	if !ok || target["engine"] != "fake" {
		t.Fatalf("target = %#v", report["target"])
	}
}

func TestInspectCommandRejectsUnsupportedFormatBeforeOpeningAdapter(t *testing.T) {
	cmd := newRootCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"inspect", "fake://local", "--format=xml", "--sample-window=1ms"})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "format") {
		t.Fatalf("Execute() error = %v; want format validation error", err)
	}
}
