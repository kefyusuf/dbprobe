package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const historyUnavailableReason = "local history unavailable; inspection was not persisted"
const historyCloseUncertainReason = "local history close failed; snapshot durability could not be confirmed"

func TestDefaultRootPersistsHistoryAcrossInvocations(t *testing.T) {
	dataRoot := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dataRoot)
	t.Setenv("LOCALAPPDATA", dataRoot)
	t.Setenv("HOME", dataRoot)

	for i := 0; i < 2; i++ {
		stdout := executeDefaultRoot(t, "inspect", "fake://local", "--format=json", "--sample-window=1ms")
		assertSchemaVersion(t, stdout, "dbprobe.inspect/v1alpha1")
	}

	stdout := executeDefaultRoot(t, "diff", "fake://local", "--format=json")
	assertSchemaVersion(t, stdout, "dbprobe.diff/v1alpha1")

	path, err := defaultHistoryPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "dbprobe.db" {
		t.Fatalf("history path=%q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("history database is empty")
	}
}

func TestInspectContinuesWhenLocalHistoryCannotOpen(t *testing.T) {
	for _, tc := range []struct {
		name string
		deps commandDependencies
	}{
		{
			name: "path resolution failure",
			deps: commandDependencies{
				historyPath: func() (string, error) { return "", errors.New("sensitive path failure") },
				openHistory: func(context.Context, string) (ownedHistoryStore, error) {
					t.Fatal("history store must not open after path resolution failure")
					return nil, nil
				},
			},
		},
		{
			name: "store open failure",
			deps: commandDependencies{
				historyPath: func() (string, error) { return filepath.Join(t.TempDir(), "dbprobe.db"), nil },
				openHistory: func(context.Context, string) (ownedHistoryStore, error) {
					return nil, errors.New("sensitive database failure")
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newRootCommandWithDependencies(tc.deps)
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"inspect", "fake://local", "--format=json", "--sample-window=1ms"})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("inspect error=%v stderr=%q", err, stderr.String())
			}
			assertHistoryWarning(t, stdout.String(), stderr.String(), historyUnavailableReason)
		})
	}
}

func TestInspectContinuesWhenLocalHistoryCannotClose(t *testing.T) {
	store := &fakeOwnedHistoryStore{closeErr: errors.New("sensitive close failure")}
	deps := commandDependencies{
		historyPath: func() (string, error) { return filepath.Join(t.TempDir(), "dbprobe.db"), nil },
		openHistory: func(context.Context, string) (ownedHistoryStore, error) {
			return store, nil
		},
	}
	cmd := newRootCommandWithDependencies(deps)
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"inspect", "fake://local", "--format=json", "--sample-window=1ms"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("inspect error=%v stderr=%q", err, stderr.String())
	}
	if store.closed != 1 {
		t.Fatalf("history close calls=%d want=1", store.closed)
	}
	assertHistoryWarning(t, stdout.String(), stderr.String(), historyCloseUncertainReason)
}

func TestDiffStillRequiresLocalHistory(t *testing.T) {
	deps := commandDependencies{
		historyPath: func() (string, error) { return filepath.Join(t.TempDir(), "dbprobe.db"), nil },
		openHistory: func(context.Context, string) (ownedHistoryStore, error) {
			return nil, errors.New("history unavailable")
		},
	}
	cmd := newRootCommandWithDependencies(deps)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"diff", "fake://local", "--format=json"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "history unavailable") {
		t.Fatalf("diff error=%v", err)
	}
}

func assertHistoryWarning(t *testing.T, stdout, stderr, expectedReason string) {
	t.Helper()
	var report struct {
		SchemaVersion string `json:"schema_version"`
		Warnings      []struct {
			CollectorID string `json:"collector_id"`
			Reason      string `json:"reason"`
		} `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if report.SchemaVersion != "dbprobe.inspect/v1alpha1" {
		t.Fatalf("schema_version=%q", report.SchemaVersion)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].CollectorID != "history" || report.Warnings[0].Reason != expectedReason {
		t.Fatalf("warnings=%#v want reason=%q", report.Warnings, expectedReason)
	}
	if strings.Contains(stdout, "sensitive") || strings.Contains(stderr, "sensitive") {
		t.Fatalf("history error details leaked: stdout=%q stderr=%q", stdout, stderr)
	}
}

func executeDefaultRoot(t *testing.T, args ...string) string {
	t.Helper()
	cmd := newRootCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(%v) error=%v stderr=%q", args, err, stderr.String())
	}
	return stdout.String()
}
