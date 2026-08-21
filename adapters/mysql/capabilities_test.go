package mysql

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/capability"
)

func TestDiscoverCapabilitiesOnlyEnablesSuccessfulProbes(t *testing.T) {
	allowed := map[string]bool{
		probePerformanceSchema: true,
		probeSessions:          true,
		probeTransactions:      true,
		probeQueryDigest:       true,
		probeIndexes:           true,
		probeObjects:           true,
		probeStorageCache:      true,
		probeExplain:           true,
		probeInnoDB:            true,
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

func TestDiscoverCapabilitiesRequiresPerformanceSchemaReadAccess(t *testing.T) {
	caps := discoverCapabilities(context.Background(), true, func(_ context.Context, query string) error {
		if query == probePerformanceSchema {
			return errors.New("select denied")
		}
		return nil
	})
	for _, unexpected := range []string{
		"mysql.performance_schema",
		"activity.sessions",
		"workload.query_summary",
		"schema.indexes",
		"locking.wait_graph",
		"replication.status",
		"storage.cache",
	} {
		if caps.Has(capability.Capability(unexpected)) {
			t.Fatalf("performance-schema-dependent capability %q claimed without read access", unexpected)
		}
	}
	for _, expected := range []string{"activity.transactions", "schema.objects", "query.explain", "mysql.innodb", "mysql.sys_schema"} {
		if !caps.Has(capability.Capability(expected)) {
			t.Fatalf("independent capability %q should still be discoverable", expected)
		}
	}
}

func TestCapabilityProbesCoverCollectorSources(t *testing.T) {
	for query, required := range map[string][]string{
		probeIndexes: {
			"information_schema.statistics",
			"performance_schema.table_io_waits_summary_by_index_usage",
		},
		probeLockWaits: {
			"performance_schema.data_lock_waits",
			"performance_schema.data_locks",
		},
		probeReplication: {
			"performance_schema.replication_connection_status",
			"performance_schema.replication_applier_status",
			"performance_schema.replication_applier_status_by_coordinator",
			"performance_schema.replication_applier_status_by_worker",
		},
	} {
		for _, source := range required {
			if !strings.Contains(query, source) {
				t.Fatalf("probe %q does not validate collector source %q", query, source)
			}
		}
	}
}
