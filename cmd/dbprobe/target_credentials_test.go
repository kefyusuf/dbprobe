package main

import (
	"net/url"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

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
