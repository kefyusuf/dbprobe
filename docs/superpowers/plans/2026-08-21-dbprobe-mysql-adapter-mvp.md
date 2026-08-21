# dbprobe MySQL Adapter MVP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the first production database adapter for MySQL 8.0 compatibility and MySQL 8.4 LTS without changing dbprobe's database-agnostic core dependency direction.

**Architecture:** All MySQL connectivity, SQL, capability probes, collectors, and engine-native rules live under `adapters/mysql`. The adapter exposes only SDK contracts to the application/core layers. Performance Schema is the primary telemetry source; `sys` is optional enrichment. Missing privileges/features reduce capabilities instead of failing the entire inspection.

**Tech Stack:** Go 1.25, `github.com/go-sql-driver/mysql v1.10.0`, MySQL 8.0.46 legacy/EOL compatibility, MySQL 8.4.11 LTS, Performance Schema, optional sys schema, Docker Compose integration tests.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Global Constraints

- `internal/core`, `internal/app`, `internal/platform`, `internal/surfaces`, and `sdk` must not import `database/sql`, `go-sql-driver/mysql`, or `adapters/mysql`.
- All production SQL for this plan lives under `adapters/mysql`.
- Adapter operations are inspection-only: no `SET GLOBAL`, `FLUSH`, `TRUNCATE`, DDL, DML, kill/session mutation, or remediation.
- Capability discovery is probe-based. Missing permissions/features produce capability gaps, not fabricated completeness.
- A Performance Schema capability means the relevant telemetry is both enabled and readable by the configured account.
- `performance_schema` is primary; `sys` is optional.
- MySQL 8.0.46 is a legacy compatibility target only because the 8.0 series reached EOL in April 2026; MySQL 8.4 LTS is the primary production target.
- MariaDB/Percona-specific compatibility is not promised.
- Query evidence uses normalized digest text only; arbitrary statement text, transaction queries, row/document data, and replication error-message text are never collected.
- Target/driver errors must never echo passwords or full raw DSNs.
- URI connection parameters use an explicit safe allowlist; arbitrary driver/system-variable parameters are rejected.
- Engine-native signals/findings use `mysql.*`; portable semantics use `core.*` only when equivalent to other engines.
- Adapter contract remains `v0.1` and is still pre-stable.

---

## Implemented MVP Surface

### Adapter/runtime
- [x] `mysql.New()` metadata and scheme matching.
- [x] Credential-safe `mysql://` URI parsing using `go-sql-driver/mysql.Config`.
- [x] Driver DSN/password remain private adapter state and are redacted from errors.
- [x] Safe URI option allowlist: `tls`, `timeout`, `readTimeout`, `writeTimeout`.
- [x] Bounded `database/sql` pool owned only by the adapter.
- [x] Stable target fingerprint from `server_uuid|database`, with host/port/database fallback.
- [x] Idempotent runtime close.

### Capabilities
- [x] Probe-based Performance Schema readability.
- [x] Sessions, transactions, query digest, indexes, schema objects, lock graph, replication, storage cache, EXPLAIN, InnoDB and optional sys capabilities.
- [x] Multi-source capabilities probe every table required by their collector.
- [x] Restricted Performance Schema access does not hide independently readable INFORMATION_SCHEMA capabilities.

### Collectors
- [x] Connection/thread/uptime gauges.
- [x] Connection/temp-table/InnoDB/statement counters with centralized delta sampling.
- [x] Bounded normalized query digest telemetry.
- [x] Table/full-scan telemetry.
- [x] Index read/metadata telemetry including structural signatures with column/expression, prefix length and direction.
- [x] Transaction age/state/row-lock metadata without `TRX_QUERY`.
- [x] Lock waiter→blocker evidence with opaque ephemeral IDs.
- [x] Replication receiver/applier/worker state and numeric error codes without `LAST_ERROR_MESSAGE`.

### Deterministic findings
- [x] `core.connection_saturation`: warn >=85%, critical >=95%.
- [x] `mysql.long_transaction`.
- [x] `mysql.lock_wait_contention`.
- [x] `mysql.query_full_scan_heavy` using sampled deltas.
- [x] `mysql.query_rows_examined_amplification` using sampled deltas.
- [x] `mysql.unused_index` as a low-confidence review candidate, never an automatic DROP recommendation.
- [x] `mysql.redundant_index` using structural signatures when available.
- [x] `mysql.buffer_pool_hit_low` with minimum sampled volume.
- [x] `mysql.replication_stopped`.
- [x] `mysql.replication_error` from numeric metadata only.

### CLI/integration
- [x] MySQL adapter registered only in `cmd/dbprobe` composition root.
- [x] Reusable adapter contract can include MySQL when `DBPROBE_TEST_MYSQL_DSN` is supplied.
- [x] Docker Compose fixture pins MySQL 8.0.46 and 8.4.11.
- [x] Fixture user has SELECT/diagnostic visibility and no application write privilege.
- [x] `make test-mysql` defines integration, contract, CLI JSON and credential-redaction acceptance checks.

---

## Verification State

Verified locally without GitHub Actions:
- [x] capability-discovery tests.
- [x] health collector tests + race detector.
- [x] query digest collector tests + race detector.
- [x] index/table collector tests + race detector for the driver-independent collector layer.
- [x] transaction/lock/replication collector tests + race detector.
- [x] deterministic findings tests + race detector, including connection threshold and semantics-aware redundant-index regression tests.

Environment-blocked gates:
- [ ] Full `go mod tidy`, `gofmt`, `go vet`, `go test ./...`, `go test -race ./...` on the complete MySQL branch with a Go 1.25 toolchain. The current container has Go 1.23 and no outbound DNS; GitHub Actions quota/billing is currently unavailable.
- [ ] `make test-mysql` against the Docker matrix. Docker/Podman is not installed in the current execution environment.

These two items are release/merge gates. Their absence must not be represented as a passing MySQL MVP acceptance test.

---

## Acceptance Gate

- [ ] `go mod tidy` produces no diff.
- [ ] `gofmt`, `go vet`, `go test ./...`, `go test -race ./...` pass under Go 1.25.x.
- [ ] Architecture dependency guard passes with the MySQL driver present only under adapter/composition-root allowed paths.
- [ ] `make test-mysql` passes against MySQL 8.0.46 and 8.4.11.
- [ ] Credential-bearing connection failures do not echo credentials to stdout/stderr.
- [ ] `dbprobe inspect mysql://... --format=json` preserves `dbprobe.inspect/v1alpha1`.
- [ ] No MySQL-specific branch appears in `internal/core`.

## Definition of Done

The MySQL adapter becomes merge-ready only after the environment-blocked acceptance gates above pass. Until then, the implementation is feature-complete at source level but remains verification-pending rather than release-ready.
