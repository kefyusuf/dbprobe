package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

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
