package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/capability"
)

func TestDiscoverCapabilitiesOnlyEnablesSuccessfulProbes(t *testing.T) {
	allowed := map[string]bool{
		probeSessions:     true,
		probeTransactions: true,
		probeQueryDigest:  true,
		probeIndexes:      true,
		probeObjects:      true,
		probeStorageCache: true,
		probeExplain:      true,
		probeInnoDB:       true,
	}
	caps := discoverCapabilities(context.Background(), true, func(_ context.Context, query string) error {
		if allowed[query] {
			return nil
		}
		return errors.New("unavailable")
	})

	for _, expected := range []string{
		"mysql.performance_schema",
		"activity.sessions",
		"activity.transactions",
		"workload.query_summary",
		"schema.indexes",
		"schema.objects",
		"storage.cache",
		"query.explain",
		"mysql.innodb",
	} {
		if !caps.Has(capability.Capability(expected)) {
			t.Fatalf("missing capability %q", expected)
		}
	}
	for _, unexpected := range []string{"locking.wait_graph", "replication.status", "mysql.replication", "mysql.sys_schema"} {
		if caps.Has(capability.Capability(unexpected)) {
			t.Fatalf("unexpected capability %q", unexpected)
		}
	}
}

func TestDiscoverCapabilitiesWithoutPerformanceSchemaDoesNotClaimIt(t *testing.T) {
	caps := discoverCapabilities(context.Background(), false, func(context.Context, string) error { return nil })
	if caps.Has(capability.Capability("mysql.performance_schema")) {
		t.Fatal("performance schema capability claimed while disabled")
	}
}
