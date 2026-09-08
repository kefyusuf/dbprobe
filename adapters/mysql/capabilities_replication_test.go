package mysql

import (
	"strings"
	"testing"
)

func TestReplicationCapabilityProbeCoversActiveLagDependencies(t *testing.T) {
	for _, table := range []string{
		"replication_connection_status",
		"replication_applier_status",
		"replication_applier_status_by_coordinator",
		"replication_applier_status_by_worker",
		"replication_applier_configuration",
	} {
		if !strings.Contains(probeReplication, table) {
			t.Fatalf("replication capability probe does not cover %s", table)
		}
	}
}
