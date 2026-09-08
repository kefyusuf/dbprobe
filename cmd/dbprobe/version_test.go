package main

import (
	"bytes"
	"testing"
)

func TestRootVersionDefaultsToDevelopmentMetadata(t *testing.T) {
	cmd := newRootCommandWithDependencies(commandDependencies{})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("--version error=%v stderr=%q", err, stderr.String())
	}

	const want = "dbprobe version dev (commit unknown, built unknown)\n"
	if got := stdout.String(); got != want {
		t.Fatalf("--version output=%q want=%q", got, want)
	}
}
