package sqlite

import (
	"strings"
	"testing"
)

func TestMigrationPlan(t *testing.T) {
	statements, err := migrationStatements(0)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(statements, "\n")
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS snapshots",
		"CREATE TABLE IF NOT EXISTS trend_metrics",
		"metric_kind TEXT NOT NULL",
		"rate_per_second REAL",
		"window_seconds REAL",
		"PRAGMA user_version = 1",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if statements, err := migrationStatements(1); err != nil || len(statements) != 0 {
		t.Fatalf("version 1 plan=%v err=%v", statements, err)
	}
	if _, err := migrationStatements(2); err == nil {
		t.Fatal("future schema version must fail closed")
	}
}

func TestBootstrapPragmas(t *testing.T) {
	joined := strings.Join(bootstrapStatements(), "\n")
	for _, required := range []string{"PRAGMA foreign_keys = ON", "PRAGMA busy_timeout = 5000"} {
		if !strings.Contains(joined, required) {
			t.Fatalf("bootstrap missing %q", required)
		}
	}
}
