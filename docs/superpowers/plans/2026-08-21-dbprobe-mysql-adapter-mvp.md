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
- Target/driver errors must never echo passwords, raw targets, or user-supplied connection option values/keys.
- Engine-native signals/findings use `mysql.*`; portable semantics use `core.*` only when equivalent to other engines.
- Adapter contract remains `v0.1` and is still pre-stable.
- CLI connection targets use the canonical `mysql://user:password@host:port/database` URI form; raw driver DSNs are not part of the public target contract.

---

## Implementation Status

Source implementation for Tasks 1-7 is complete on `feat/mysql-adapter-mvp`.

Verified without GitHub Actions:

- collector package unit tests;
- collector race tests;
- deterministic finding unit/race tests;
- capability discovery harness tests;
- semantics-aware index signature tests;
- architecture source review and dependency-boundary review.

Pending environment gates:

- full Go 1.25 `go mod tidy` / `gofmt` / `go vet` / `go test ./...` / `go test -race ./...`;
- Docker integration against pinned MySQL 8.0.46 and 8.4.11.

The pending status is environmental: GitHub Actions quota currently prevents jobs from starting, the execution container has no Docker runtime, and outbound DNS is unavailable for installing a separate Go 1.25 toolchain.

---

## Completed Source Tasks

### Task 1 — Adapter identity and safe connection parsing

- [x] `mysql.New()` adapter metadata/matching.
- [x] Canonical MySQL URI parsing through `go-sql-driver/mysql` config.
- [x] Credentials remain private to the adapter.
- [x] Connection option whitelist: `tls`, `timeout`, `readTimeout`, `writeTimeout`.
- [x] Risk-expanding/unknown driver options rejected without echoing values or keys.
- [x] Error redaction for raw targets/DSNs/passwords.

### Task 2 — Runtime, identity and capabilities

- [x] Adapter-owned bounded `database/sql` pool.
- [x] Stable UUID+database fingerprint with host/port fallback.
- [x] Probe-based capability discovery.
- [x] `mysql.performance_schema` requires both engine enablement and successful baseline reads.
- [x] Compound lock/replication/index probes cover the tables used by their collectors.
- [x] Idempotent `Close()`.

### Task 3 — Health signals

- [x] `core.connections.used`, `core.connections.limit`, `mysql.threads.running`.
- [x] Cumulative connection/temp-table/InnoDB/statement counters.
- [x] Server uptime evidence for low-confidence unused-index review.

### Task 4 — Workload, table and index signals

- [x] Bounded normalized statement digest collection.
- [x] Calls, latency, examined/sent rows, no-index, temp-table, error/warning counters.
- [x] Query-shape only; literals/user rows not collected.
- [x] Stable index/table object identities.
- [x] Index structural signature includes prefix length, expression and direction.
- [x] Table scan collector requires the capability that proves index-usage telemetry is readable.

### Task 5 — Transactions, locks and replication

- [x] InnoDB transaction age/state/rows-locked/rows-modified without `TRX_QUERY`.
- [x] Requester→blocker edges from Performance Schema locking tables.
- [x] Ephemeral transaction/lock IDs are evidence, not stable finding identity.
- [x] Replication receiver/applier/worker state and numeric error codes.
- [x] `LAST_ERROR_MESSAGE` is never selected.

### Task 6 — Deterministic findings

- [x] `core.connection_saturation` (warn >=85%, critical >=95%).
- [x] `mysql.long_transaction`.
- [x] `mysql.lock_wait_contention`.
- [x] `mysql.query_full_scan_heavy`.
- [x] `mysql.query_rows_examined_amplification`.
- [x] `mysql.unused_index` as low-confidence review candidate only.
- [x] `mysql.redundant_index` using structural signatures when available.
- [x] `mysql.buffer_pool_hit_low` using sampled deltas and a traffic floor.
- [x] `mysql.replication_stopped`.
- [x] `mysql.replication_error` based on numeric error codes only.

### Task 7 — CLI and integration fixture

- [x] `mysql.New()` registered only in CLI composition root.
- [x] Adapter contract table includes MySQL metadata/match coverage without requiring a live server in default unit tests.
- [x] Pinned Docker services for MySQL 8.0.46 and 8.4.11.
- [x] Read-only fixture account with SELECT + PROCESS diagnostic visibility.
- [x] Integration flow covers read workload, denied write, stable fingerprint, capabilities, evidence and shared JSON schema.
- [x] `make test-mysql` entrypoint plus CLI credential-redaction smoke checks.

---

## Task 8 — Acceptance Gate (Pending Environment)

- [ ] `go mod tidy` produces no diff.
- [ ] `gofmt`, `go vet`, `go test ./...`, `go test -race ./...` pass on Go 1.25.
- [ ] Architecture dependency guard passes with MySQL driver present only under adapter/composition-root allowed paths.
- [ ] `make test-mysql` passes against MySQL 8.0.46 and 8.4.11.
- [ ] Credential-bearing failing target does not echo credentials to stdout/stderr.
- [ ] `dbprobe inspect mysql://... --format=json` preserves `dbprobe.inspect/v1alpha1`.
- [ ] No MySQL-specific branch appears in `internal/core`.

## Definition of Done

The MySQL adapter becomes merge-ready only after Task 8 is fully executed in an environment with Go 1.25 and Docker. Until then the source MVP is implemented but acceptance remains explicitly pending.
