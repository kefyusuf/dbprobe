package sqlite

import (
	"context"
	"database/sql/driver"
	"os"
	"path/filepath"
	"testing"
)

type fakeConnector struct{ state *fakeSQLiteState }

func (c fakeConnector) Connect(context.Context) (driver.Conn, error) {
	return &fakeSQLiteConn{state: c.state}, nil
}
func (c fakeConnector) Driver() driver.Driver { return fakeSQLiteDriver{state: c.state} }

func TestOpenCreatesOwnedSingleConnectionStore(t *testing.T) {
	state := newFakeSQLiteState(0)
	root := t.TempDir()
	path := filepath.Join(root, "nested", "dbprobe.db")
	var gotDSN string
	opened, err := Open(context.Background(), path, func(dsn string) (driver.Connector, error) {
		gotDSN = dsn
		return fakeConnector{state: state}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotDSN != path {
		t.Fatalf("dsn=%q want=%q", gotDSN, path)
	}
	if opened == nil || opened.Store == nil || opened.db == nil {
		t.Fatalf("opened=%#v", opened)
	}
	if got := opened.db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("max open connections=%d; want 1", got)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("parent directory: %v", err)
	}
	if err := opened.Close(); err != nil {
		t.Fatal(err)
	}
	if err := opened.Close(); err != nil {
		t.Fatalf("second Close() error=%v", err)
	}
}

func TestOpenRejectsInvalidInputs(t *testing.T) {
	if _, err := Open(context.Background(), "", func(string) (driver.Connector, error) { return nil, nil }); err == nil {
		t.Fatal("expected empty path error")
	}
	if _, err := Open(context.Background(), "dbprobe.db", nil); err == nil {
		t.Fatal("expected nil connector factory error")
	}
}

func TestOpenRestrictsExistingDatabaseFilePermissions(t *testing.T) {
	state := newFakeSQLiteState(1)
	path := filepath.Join(t.TempDir(), "dbprobe.db")
	if err := os.WriteFile(path, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	opened, err := Open(context.Background(), path, func(string) (driver.Connector, error) {
		return fakeConnector{state: state}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode=%#o want=%#o", got, 0o600)
	}
}
