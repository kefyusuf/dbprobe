package mysql

import (
	"context"

	"github.com/kefyusuf/dbprobe/sdk/capability"
)

const (
	probePerformanceSchema = `SELECT
  EXISTS(SELECT 1 FROM performance_schema.global_status LIMIT 1),
  EXISTS(SELECT 1 FROM performance_schema.global_variables LIMIT 1)`
	probeSessions     = "SELECT 1 FROM performance_schema.threads LIMIT 1"
	probeTransactions = "SELECT COUNT(*) FROM information_schema.innodb_trx"
	probeQueryDigest  = "SELECT COUNT(*) FROM performance_schema.events_statements_summary_by_digest"
	probeIndexes      = `SELECT COUNT(*)
FROM information_schema.statistics s
LEFT JOIN performance_schema.table_io_waits_summary_by_index_usage p
  ON p.OBJECT_SCHEMA = s.TABLE_SCHEMA
 AND p.OBJECT_NAME = s.TABLE_NAME
 AND p.INDEX_NAME = s.INDEX_NAME
WHERE s.TABLE_SCHEMA = DATABASE()`
	probeObjects = `SELECT COUNT(*)
FROM information_schema.tables t
LEFT JOIN information_schema.columns c
  ON c.TABLE_SCHEMA = t.TABLE_SCHEMA
 AND c.TABLE_NAME = t.TABLE_NAME
LEFT JOIN information_schema.statistics s
  ON s.TABLE_SCHEMA = t.TABLE_SCHEMA
 AND s.TABLE_NAME = t.TABLE_NAME
WHERE t.TABLE_SCHEMA = DATABASE()`
	probeSchemaFingerprint = `SELECT
  EXISTS(SELECT 1 FROM information_schema.tables WHERE TABLE_SCHEMA = DATABASE() LIMIT 1),
  EXISTS(SELECT 1 FROM information_schema.columns WHERE TABLE_SCHEMA = DATABASE() LIMIT 1),
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE TABLE_SCHEMA = DATABASE() LIMIT 1),
  EXISTS(SELECT 1 FROM information_schema.table_constraints WHERE CONSTRAINT_SCHEMA = DATABASE() LIMIT 1),
  EXISTS(SELECT 1 FROM information_schema.key_column_usage WHERE CONSTRAINT_SCHEMA = DATABASE() LIMIT 1)`
	probeLockWaits = `SELECT COUNT(*)
FROM performance_schema.data_lock_waits w
LEFT JOIN performance_schema.data_locks l
  ON l.ENGINE = w.ENGINE
 AND l.ENGINE_LOCK_ID = w.REQUESTING_ENGINE_LOCK_ID`
	probeReplication = `SELECT COUNT(*)
FROM performance_schema.replication_connection_status c
LEFT JOIN performance_schema.replication_applier_status a
  ON a.CHANNEL_NAME = c.CHANNEL_NAME
LEFT JOIN performance_schema.replication_applier_status_by_coordinator co
  ON co.CHANNEL_NAME = c.CHANNEL_NAME
LEFT JOIN performance_schema.replication_applier_status_by_worker w
  ON w.CHANNEL_NAME = c.CHANNEL_NAME
LEFT JOIN performance_schema.replication_applier_configuration cfg
  ON cfg.CHANNEL_NAME = c.CHANNEL_NAME`
	probeStorageCache   = "SELECT COUNT(*) FROM performance_schema.global_status WHERE VARIABLE_NAME IN ('Innodb_buffer_pool_reads','Innodb_buffer_pool_read_requests')"
	probeExplain        = "EXPLAIN SELECT 1"
	probeInnoDB         = "SELECT COUNT(*) FROM information_schema.engines WHERE engine = 'InnoDB' AND support IN ('YES','DEFAULT')"
	probeInnoDBMetrics  = "SELECT COUNT FROM INFORMATION_SCHEMA.INNODB_METRICS WHERE NAME = 'trx_rseg_history_len' LIMIT 1"
	probeSys            = "SELECT 1 FROM sys.version LIMIT 1"
)

type probeFunc func(context.Context, string) error

func discoverCapabilities(ctx context.Context, performanceSchema bool, probe probeFunc) capability.Set {
	values := make([]capability.Capability, 0, 14)
	addIf := func(query string, caps ...capability.Capability) {
		if err := probe(ctx, query); err == nil {
			values = append(values, caps...)
		}
	}

	performanceSchemaReadable := false
	if performanceSchema {
		if err := probe(ctx, probePerformanceSchema); err == nil {
			performanceSchemaReadable = true
			values = append(values, "mysql.performance_schema")
		}
	}

	if performanceSchemaReadable {
		addIf(probeSessions, "activity.sessions")
		addIf(probeQueryDigest, "workload.query_summary")
		addIf(probeIndexes, "schema.indexes")
		addIf(probeLockWaits, "locking.wait_graph")
		addIf(probeReplication, "replication.status", "mysql.replication")
		addIf(probeStorageCache, "storage.cache")
	}

	addIf(probeTransactions, "activity.transactions")
	addIf(probeObjects, "schema.objects")
	addIf(probeSchemaFingerprint, "mysql.schema_fingerprint")
	addIf(probeExplain, "query.explain")
	addIf(probeInnoDB, "mysql.innodb")
	addIf(probeInnoDBMetrics, "mysql.innodb_metrics")
	addIf(probeSys, "mysql.sys_schema")

	return capability.New(values...)
}
