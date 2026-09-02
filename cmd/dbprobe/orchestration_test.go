package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kefyusuf/dbprobe/internal/platform/baseline"
)

type closableMemoryHistory struct {
	*baseline.Memory
	onClose func()
}

func (s *closableMemoryHistory) Close() error {
	if s.onClose != nil {
		s.onClose()
	}
	return nil
}

func TestHistoryDependenciesOrchestrateInspectThenDiff(t *testing.T) {
	memory := baseline.NewMemory()
	path := filepath.Join(t.TempDir(), "dbprobe.db")
	opens := 0
	closes := 0

	deps := commandDependencies{
		historyPath: func() (string, error) { return path, nil },
		openHistory: func(_ context.Context, gotPath string) (ownedHistoryStore, error) {
			opens++
			if gotPath != path {
				t.Fatalf("history path=%q want=%q", gotPath, path)
			}
			return &closableMemoryHistory{
				Memory:  memory,
				onClose: func() { closes++ },
			}, nil
		},
	}

	for i := 0; i < 2; i++ {
		stdout := executeRoot(t, deps, "inspect", "fake://local", "--format=json", "--sample-window=1ms")
		assertSchemaVersion(t, stdout, "dbprobe.inspect/v1alpha1")
	}

	stdout := executeRoot(t, deps, "diff", "fake://local", "--format=json")
	assertSchemaVersion(t, stdout, "dbprobe.diff/v1alpha1")

	if opens != 3 || closes != 3 {
		t.Fatalf("history lifecycle opens=%d closes=%d; want 3/3", opens, closes)
	}
}

func TestHistoryBackedCommandsValidateBeforeOpeningStore(t *testing.T) {
	opens := 0
	deps := commandDependencies{
		historyPath: func() (string, error) { return "/tmp/dbprobe.db", nil },
		openHistory: func(context.Context, string) (ownedHistoryStore, error) {
			opens++
			return &closableMemoryHistory{Memory: baseline.NewMemory()}, nil
		},
	}

	for _, args := range [][]string{
		{"inspect", "fake://local", "--format=xml", "--sample-window=1ms"},
		{"diff", "fake://local", "--format=xml"},
	} {
		cmd := newRootCommandWithDependencies(deps)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "format") {
			t.Fatalf("args=%v err=%v; want format validation error", args, err)
		}
	}

	if opens != 0 {
		t.Fatalf("history store opened %d times before command validation", opens)
	}
}

func executeRoot(t *testing.T, deps commandDependencies, args ...string) string {
	t.Helper()
	cmd := newRootCommandWithDependencies(deps)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(%v) error=%v stderr=%q", args, err, stderr.String())
	}
	return stdout.String()
}

func assertSchemaVersion(t *testing.T, raw, want string) {
	t.Helper()
	var report map[string]any
	if err := json.Unmarshal([]byte(raw), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, raw)
	}
	if report["schema_version"] != want {
		t.Fatalf("schema_version=%#v want=%q", report["schema_version"], want)
	}
}
