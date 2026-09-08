package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootCommandPrintsDevelopmentVersion(t *testing.T) {
	cmd := newRootCommandWithDependencies(commandDependencies{})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr=%q", err, stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "dbprobe version dev" {
		t.Fatalf("version output = %q; want %q", got, "dbprobe version dev")
	}
}
