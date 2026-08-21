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
		probePerformanceSchema:  true,
		probeSessions:           true,
		probeTransactions:       true,
		probeQueryDigest:        true,
		probeIndexes:            true,
		probeObjects:            true,
		probeSchemaFingerprint:  true,
		probeStorageCache:       true,
		probeExplain:            true,
		probeInnoDB:             true,
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
		"mysql.schema_fingerprint",
		"storage.cache",
		"query.explain",
		"mysql.innodb",
	} {
		if !caps.Has(capability.Capability(expected)) {
			t.Fatalf("missing capability %q", expected)
		}
	}
	for _, unexpected := range []string{"locking.wait_graph", "replication.status", "mysql.replication", "mysql.sys_schema", "mysql.innodb_metrics"} {
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
	for _, expected := range []string{"activity.transactions", "schema.objects", "mysql.schema_fingerprint", "query.explain", "mysql.innodb", "mysql.innodb_metrics", "mysql.sys_schema"} {
		if !caps.Has(capability.Capability(expected)) {
			t.Fatalf("independent capability %q should still be discoverable", expected)
		}
	}
}

func TestSchemaFingerprintCapabilityIsIndependentFromSchemaObjects(t *testing.T) {
	caps := discoverCapabilities(context.Background(), false, func(_ context.Context, query string) error {
		if query == probeSchemaFingerprint {
			return errors.New("fingerprint metadata denied")
		}
		return nil
	})
	if !caps.Has(capability.Capability("schema.objects")) {
		t.Fatal("schema.objects should remain available")
	}
	if caps.Has(capability.Capability("mysql.schema_fingerprint")) {
		t.Fatal("schema fingerprint capability claimed without full metadata visibility")
	}
}

func TestCapabilityProbesCoverCollectorSources(t *testing.T) {
	for query, required := range map[string][]string{
		probeIndexes: {
			"information_schema.statistics",
			"performance_schema.table_io_waits_summary_by_index_usage",
		},
		probeSchemaFingerprint: {
			"information_schema.tables",
			"information_schema.columns",
			"information_schema.statistics",
			"information_schema.table_constraints",
			"information_schema.key_column_usage",
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
			"performance_schema.replication_applier_configuration",
		},
	} {
		for _, source := range required {
			if !strings.Contains(query, source) {
				t.Fatalf("probe %q does not validate collector source %q", query, source)
			}
		}
	}
}
