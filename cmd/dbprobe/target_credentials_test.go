package main

import (
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

type countingReader struct {
	r     io.Reader
	bytes int
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.bytes += n
	return n, err
}

func TestResolveCommandTargetRejectsPasswordInArgument(t *testing.T) {
	raw := "mysql://dbprobe:do-not-leak@127.0.0.1:3306/shop?tls=false"
	_, err := resolveCommandTarget(raw, false, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected credential-bearing argv target to be rejected")
	}
	if strings.Contains(err.Error(), "do-not-leak") || strings.Contains(err.Error(), raw) {
		t.Fatalf("credential rejection leaked target: %q", err)
	}
}

func TestResolveCommandTargetRejectsEncodedPasswordDelimiterInUsername(t *testing.T) {
	raw := "mysql://dbprobe%3Aargv-secret@127.0.0.1:3306/shop?tls=false"
	_, err := resolveCommandTarget(raw, false, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected delimiter-bearing MySQL username to be rejected")
	}
	if strings.Contains(err.Error(), "argv-secret") || strings.Contains(err.Error(), raw) {
		t.Fatalf("username rejection leaked target: %q", err)
	}
}

func TestResolveCommandTargetReadsMySQLPasswordFromStdin(t *testing.T) {
	raw := "mysql://dbprobe@127.0.0.1:3306/shop?tls=false"
	got, err := resolveCommandTarget(raw, true, strings.NewReader("stdin-secret\n"))
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	password, ok := u.User.Password()
	if !ok || u.User.Username() != "dbprobe" || password != "stdin-secret" {
		t.Fatalf("resolved userinfo username=%q password_present=%v password=%q", u.User.Username(), ok, password)
	}
	if u.Host != "127.0.0.1:3306" || u.Path != "/shop" || u.Query().Get("tls") != "false" {
		t.Fatalf("resolved target changed connection identity: %q", got)
	}
}

func TestResolveCommandTargetBoundsPasswordInput(t *testing.T) {
	input := &countingReader{r: strings.NewReader(strings.Repeat("x", maxMySQLPasswordBytes+8192) + "\n")}
	_, err := resolveCommandTarget("mysql://dbprobe@127.0.0.1:3306/shop?tls=false", true, input)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error=%v", err)
	}
	if input.bytes > maxMySQLPasswordBytes+1 {
		t.Fatalf("password reader consumed %d bytes; want at most %d", input.bytes, maxMySQLPasswordBytes+1)
	}
}

func TestResolveCommandTargetRequiresUsernameForPasswordStdin(t *testing.T) {
	_, err := resolveCommandTarget("mysql://127.0.0.1:3306/shop?tls=false", true, strings.NewReader("secret\n"))
	if err == nil || !strings.Contains(err.Error(), "username") {
		t.Fatalf("error=%v", err)
	}
}

func TestResolveCommandTargetRejectsPasswordStdinForNonMySQLTarget(t *testing.T) {
	_, err := resolveCommandTarget("fake://local", true, strings.NewReader("secret\n"))
	if err == nil || !strings.Contains(err.Error(), "MySQL") {
		t.Fatalf("error=%v", err)
	}
}

func TestDatabaseCommandsExposePasswordStdinFlag(t *testing.T) {
	commands := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "inspect", cmd: newInspectCommandWithDependencies(commandDependencies{})},
		{name: "diff", cmd: newDiffCommand(commandDependencies{})},
		{name: "explain", cmd: newExplainCommand()},
	}
	for _, tc := range commands {
		if tc.cmd.Flags().Lookup("password-stdin") == nil {
			t.Fatalf("%s command is missing --password-stdin", tc.name)
		}
	}
}
