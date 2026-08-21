# dbprobe MySQL Adapter MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first production database adapter for MySQL 8.0 and 8.4 without changing dbprobe's database-agnostic core dependency direction.

**Architecture:** All MySQL connectivity, SQL, capability probes, collectors, and engine-native rules live under `adapters/mysql`. The adapter exposes only SDK contracts to the application/core layers. Performance Schema is the primary telemetry source; `sys` is optional enrichment. Missing privileges/features reduce capabilities instead of failing the entire inspection.

**Tech Stack:** Go 1.25, `github.com/go-sql-driver/mysql v1.10.0`, MySQL 8.0/8.4, Performance Schema, optional sys schema, Docker Compose integration tests.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Global Constraints

- `internal/core`, `internal/app`, `internal/platform`, `internal/surfaces`, and `sdk` must not import `database/sql`, `go-sql-driver/mysql`, or `adapters/mysql`.
- All production SQL for this plan lives under `adapters/mysql`.
- Adapter operations are inspection-only: no `SET GLOBAL`, `FLUSH`, `TRUNCATE`, DDL, DML, kill/session mutation, or remediation.
- Capability discovery is probe-based. Missing permissions/features produce capability gaps, not fabricated completeness.
- `performance_schema` is primary; `sys` is optional.
- MySQL 8.0 and MySQL 8.4 LTS are the compatibility targets. MariaDB/Percona-specific compatibility is not promised.
- Query text exposed as evidence must use normalized digest text; row/document data is never collected.
- Target/driver errors must never echo passwords or full raw DSNs.
- Engine-native signals/findings use `mysql.*`; portable semantics use `core.*` only when equivalent to other engines.
- Adapter contract remains `v0.1` and is still pre-stable.

---

## File Structure

```text
adapters/mysql/
├── adapter.go
├── adapter_test.go
├── config.go
├── config_test.go
├── runtime.go
├── runtime_test.go
├── capabilities.go
├── capabilities_test.go
├── queries.go
├── scan.go
├── collectors/
│   ├── server.go
│   ├── health.go
│   ├── queries.go
│   ├── indexes.go
│   ├── tables.go
│   ├── locks.go
│   ├── transactions.go
│   └── replication.go
└── findings/
    ├── connections.go
    ├── queries.go
    ├── indexes.go
    ├── transactions.go
    └── replication.go

test/integration/mysql/
├── docker-compose.yml
├── init.sql
└── mysql_integration_test.go
```

---

### Task 1: MySQL adapter identity, safe DSN handling, and driver dependency

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `adapters/mysql/config.go`
- Create: `adapters/mysql/config_test.go`
- Create: `adapters/mysql/adapter.go`
- Create: `adapters/mysql/adapter_test.go`

**Interfaces:**
- Produces: `mysql.New() adapter.Adapter`, `mysql.ParseConfig(raw string) (Config, error)`.

- [ ] Write failing tests asserting: schemes `mysql://` and driver DSN are accepted; adapter ID is `mysql`; malformed targets fail without including passwords; target display name never includes credentials.
- [ ] Run `go test ./adapters/mysql -v` and verify RED.
- [ ] Add `github.com/go-sql-driver/mysql v1.10.0` and implement config parsing. URI form is converted through `mysql.Config`; driver DSN remains supported for CLI compatibility.
- [ ] `adapter.Match` accepts only scheme `mysql`; URI target parsing must not expose raw credentials in errors.
- [ ] Run adapter tests, `go vet ./...`, then commit `feat: add MySQL adapter identity and safe config`.

---

### Task 2: Connection runtime, server identity, and capability discovery

**Files:**
- Create: `adapters/mysql/runtime.go`
- Create: `adapters/mysql/runtime_test.go`
- Create: `adapters/mysql/capabilities.go`
- Create: `adapters/mysql/capabilities_test.go`
- Create: `adapters/mysql/queries.go`
- Create: `adapters/mysql/scan.go`

**Interfaces:**
- Runtime owns `*sql.DB` privately.
- Produces target metadata and capabilities through the existing SDK `adapter.Runtime` contract.

- [ ] Write sqlmock-like tests using a minimal internal query interface (do not add a mocking framework) for server metadata and probe behavior.
- [ ] Runtime opens with bounded pool defaults (`MaxOpenConns=4`, `MaxIdleConns=2`, connection max lifetime 5m) and `PingContext` timeout.
- [ ] Read identity with a single safe query returning `VERSION()`, `@@version_comment`, `DATABASE()`, `@@server_uuid`, `@@performance_schema`, `@@read_only`, and `@@super_read_only` where available.
- [ ] Fingerprint seed is `server_uuid|database`; fallback is normalized host|port|database only when UUID is unavailable.
- [ ] Capability probes are direct SELECT probes and classify permission/missing-object errors as unavailable rather than fatal.
- [ ] Initial portable capabilities: `activity.sessions`, `activity.transactions`, `workload.query_summary`, `schema.indexes`, `schema.objects`, `locking.wait_graph`, `replication.status`, `storage.cache`, `query.explain`.
- [ ] Initial engine capabilities: `mysql.performance_schema`, `mysql.sys_schema`, `mysql.innodb`, `mysql.replication`.
- [ ] Verify tests and commit `feat: discover MySQL runtime capabilities`.

---

### Task 3: Server and health collectors

**Files:**
- Create: `adapters/mysql/collectors/server.go`
- Create: `adapters/mysql/collectors/health.go`
- Add tests next to collectors.

**Signals:**
- snapshot: `core.connections.used`, `core.connections.limit`, `mysql.threads.running`
- counters: `mysql.connections.total`, `mysql.transactions.commits`, `mysql.transactions.rollbacks`, `mysql.temp.disk_tables`, `mysql.innodb.buffer_pool.read_requests`, `mysql.innodb.buffer_pool.reads`, `mysql.innodb.row_lock_waits`, `mysql.innodb.log_waits`

- [ ] Write failing collector tests using deterministic row fixtures.
- [ ] Read values from `performance_schema.global_status` and `performance_schema.global_variables`; never use mutation-oriented SHOW/FLUSH paths.
- [ ] Mark counters `ExactnessCumulative`; gauges `ExactnessScraped`.
- [ ] Return unavailable/missing metrics as omitted observations with collector warning rather than zero-filling.
- [ ] Verify planner double-sampling produces rates without MySQL-specific sleep logic.
- [ ] Commit `feat: collect MySQL server health signals`.

---

### Task 4: Workload, table, and index collectors

**Files:**
- Create: `adapters/mysql/collectors/queries.go`
- Create: `adapters/mysql/collectors/indexes.go`
- Create: `adapters/mysql/collectors/tables.go`
- Add tests next to collectors.

**Data sources:**
- `performance_schema.events_statements_summary_by_digest`
- `performance_schema.table_io_waits_summary_by_index_usage`
- optional `sys.schema_unused_indexes`, `sys.schema_redundant_indexes`, `sys.schema_tables_with_full_table_scans`

- [ ] Query collector emits bounded top-N normalized digests with calls, total/mean latency, rows examined/sent, no-index counters, temp-table counters, errors/warnings.
- [ ] Query object identity is stable `mysql.query:<schema>:<digest>`; literal SQL values are not collected.
- [ ] Index/table collectors expose stable schema-qualified object identities and observation windows as cumulative evidence.
- [ ] `sys` unavailable must fall back to Performance Schema/INFORMATION_SCHEMA where the equivalent signal is safe; otherwise capability/report says unavailable.
- [ ] Enforce hard row limits on all top-N diagnostics.
- [ ] Verify tests and commit `feat: collect MySQL workload and schema signals`.

---

### Task 5: Transactions, lock graph, and replication collectors

**Files:**
- Create: `adapters/mysql/collectors/transactions.go`
- Create: `adapters/mysql/collectors/locks.go`
- Create: `adapters/mysql/collectors/replication.go`
- Add tests next to collectors.

**Data sources:**
- `information_schema.innodb_trx`
- `performance_schema.data_locks`
- `performance_schema.data_lock_waits`
- `performance_schema.replication_connection_status`
- `performance_schema.replication_applier_status`
- `performance_schema.replication_applier_status_by_worker`

- [ ] Long transaction observations include age/state/rows locked but never arbitrary row data.
- [ ] Lock collector derives requester→blocker edges from `data_lock_waits`; lock IDs remain opaque and are not treated as stable suppression identities.
- [ ] Replication collector handles no-replica rows as normal absence, not error.
- [ ] Replication errors are represented as metadata evidence with bounded/sanitized message text.
- [ ] Verify tests and commit `feat: collect MySQL transaction lock and replication state`.

---

### Task 6: Initial deterministic MySQL findings

**Files:**
- Create: `adapters/mysql/findings/connections.go`
- Create: `adapters/mysql/findings/queries.go`
- Create: `adapters/mysql/findings/indexes.go`
- Create: `adapters/mysql/findings/transactions.go`
- Create: `adapters/mysql/findings/replication.go`
- Add rule tests.

**Initial finding IDs:**
- `core.connection_saturation`
- `mysql.long_transaction`
- `mysql.lock_wait_contention`
- `mysql.query_full_scan_heavy`
- `mysql.query_rows_examined_amplification`
- `mysql.unused_index`
- `mysql.redundant_index`
- `mysql.buffer_pool_hit_low`
- `mysql.replication_stopped`
- `mysql.replication_error`

- [ ] Every rule is deterministic and declares required capabilities.
- [ ] Unused-index confidence is capped when server/performance-schema observation window is young.
- [ ] Buffer-pool hit finding requires minimum read-request volume to avoid noise.
- [ ] Connection saturation thresholds: warn >= 85%, critical >= 95%.
- [ ] Findings include concrete evidence observations and never synthesize facts absent from collection.
- [ ] Verify rules and commit `feat: add initial MySQL deterministic findings`.

---

### Task 7: CLI registration and MySQL 8.0/8.4 integration matrix

**Files:**
- Modify: `cmd/dbprobe/inspect.go`
- Modify: `test/contract/adapter_contract_test.go`
- Create: `test/integration/mysql/docker-compose.yml`
- Create: `test/integration/mysql/init.sql`
- Create: `test/integration/mysql/mysql_integration_test.go`
- Modify: `Makefile`

- [ ] Register `mysql.New()` only in the CLI composition root.
- [ ] Add MySQL adapter to reusable adapter contract suite without weakening assertions.
- [ ] Compose services `mysql80` and `mysql84`; initialize a minimal workload schema and read-only dbprobe user.
- [ ] Integration assertions: connect, fingerprint stable, expected capabilities, no write privilege, server/health/query collectors return evidence, inspect JSON contract remains `dbprobe.inspect/v1alpha1`.
- [ ] Integration tests are opt-in locally (`make test-mysql`) and may become CI matrix once Actions budget permits.
- [ ] Commit `test: add MySQL adapter integration matrix`.

---

### Task 8: MySQL MVP acceptance gate

- [ ] `go mod tidy` produces no diff.
- [ ] `gofmt`, `go vet`, `go test ./...`, `go test -race ./...` pass.
- [ ] Architecture dependency guard passes with the MySQL driver present only under adapter/composition-root allowed paths.
- [ ] Docker integration passes against MySQL 8.0 and 8.4 when Docker is available.
- [ ] A credential-bearing failing DSN does not echo credentials to stdout/stderr.
- [ ] `dbprobe inspect mysql://... --format=json` preserves the shared report schema.
- [ ] No MySQL-specific branch appears in `internal/core`.

## Definition of Done

The MySQL adapter is considered MVP-ready when a read-only user can inspect MySQL 8.0 and 8.4, collect bounded deterministic health/workload/locking/replication evidence, produce initial deterministic findings, and render through the unchanged shared CLI/JSON runtime without leaking MySQL dependencies into core.