package mysql

import (
	"context"

	"github.com/kefyusuf/dbprobe/sdk/capability"
)

const (
	probeSessions     = "SELECT 1 FROM performance_schema.threads LIMIT 1"
	probeTransactions = "SELECT COUNT(*) FROM information_schema.innodb_trx"
	probeQueryDigest  = "SELECT COUNT(*) FROM performance_schema.events_statements_summary_by_digest"
	probeIndexes      = "SELECT COUNT(*) FROM performance_schema.table_io_waits_summary_by_index_usage"
	probeObjects      = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()"
	probeLockWaits    = "SELECT COUNT(*) FROM performance_schema.data_lock_waits"
	probeReplication  = "SELECT COUNT(*) FROM performance_schema.replication_connection_status"
	probeStorageCache = "SELECT COUNT(*) FROM performance_schema.global_status WHERE VARIABLE_NAME IN ('Innodb_buffer_pool_reads','Innodb_buffer_pool_read_requests')"
	probeExplain      = "EXPLAIN SELECT 1"
	probeInnoDB       = "SELECT COUNT(*) FROM information_schema.engines WHERE engine = 'InnoDB' AND support IN ('YES','DEFAULT')"
	probeSys          = "SELECT 1 FROM sys.version LIMIT 1"
)

type probeFunc func(context.Context, string) error

func discoverCapabilities(ctx context.Context, performanceSchema bool, probe probeFunc) capability.Set {
	values := make([]capability.Capability, 0, 12)
	addIf := func(query string, caps ...capability.Capability) {
		if err := probe(ctx, query); err == nil {
			values = append(values, caps...)
		}
	}

	if performanceSchema {
		values = append(values, "mysql.performance_schema")
		addIf(probeSessions, "activity.sessions")
		addIf(probeQueryDigest, "workload.query_summary")
		addIf(probeIndexes, "schema.indexes")
		addIf(probeLockWaits, "locking.wait_graph")
		addIf(probeReplication, "replication.status", "mysql.replication")
		addIf(probeStorageCache, "storage.cache")
	}

	addIf(probeTransactions, "activity.transactions")
	addIf(probeObjects, "schema.objects")
	addIf(probeExplain, "query.explain")
	addIf(probeInnoDB, "mysql.innodb")
	addIf(probeSys, "mysql.sys_schema")

	return capability.New(values...)
}
