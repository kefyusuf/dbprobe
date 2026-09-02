package main

import (
	"path/filepath"
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

func TestHistoryPathUsesDbprobeDirectory(t *testing.T) {
	got, err := historyPath("/tmp/config")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/tmp/config", "dbprobe", "history.db")
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
	if _, err := historyPath("   "); err == nil {
		t.Fatal("expected empty config dir error")
	}
}

func TestQueryRegressionMetricsAreChosenOnlyInCompositionRoot(t *testing.T) {
	mysql := queryRegressionMetrics(adapter.TargetMetadata{Engine: "mysql"})
	if mysql == nil || mysql.CallsKey != "core.query.calls" || mysql.TotalLatencyKey != "mysql.query.total_latency_ms" {
		t.Fatalf("mysql metrics=%#v", mysql)
	}
	if got := queryRegressionMetrics(adapter.TargetMetadata{Engine: "mongodb"}); got != nil {
		t.Fatalf("mongodb metrics=%#v; want nil until production profile is registered", got)
	}
}
