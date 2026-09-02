package sqlite

import (
	"context"
	"database/sql/driver"
	"os"
	"path/filepath"
	"testing"
)

type securityConnector struct{ state *fakeSQLiteState }

func (c securityConnector) Connect(context.Context) (driver.Conn, error) {
	return &fakeSQLiteConn{state: c.state}, nil
}
func (c securityConnector) Driver() driver.Driver { return fakeSQLiteDriver{state: c.state} }

func TestOpenSecuresExistingFileBeforeFactory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.db")
	if err := os.WriteFile(path, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	state := newFakeSQLiteState(0)
	opened, err := Open(context.Background(), path, func(got string) (driver.Connector, error) {
		info, err := os.Stat(got)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode at factory=%#o want 0600", info.Mode().Perm())
		}
		return securityConnector{state: state}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
}

func TestOpenRejectsExistingSymlinkBeforeFactory(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.db")
	link := filepath.Join(root, "history.db")
	if err := os.WriteFile(target, []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	calls := 0
	_, err := Open(context.Background(), link, func(string) (driver.Connector, error) {
		calls++
		return securityConnector{state: newFakeSQLiteState(0)}, nil
	})
	if err == nil {
		t.Fatal("expected symlink rejection")
	}
	if calls != 0 {
		t.Fatalf("factory calls=%d", calls)
	}
}
