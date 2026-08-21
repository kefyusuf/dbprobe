# dbprobe SQLite Baseline Pre-Driver Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement and verify the driver-independent SQLite persistence boundary—schema, migrations, snapshot codec, SQL store behavior contracts, and trend-row extraction—while deferring only concrete Go SQLite driver registration/live reopen tests until dependency access is available.

**Architecture:** `internal/platform/sqlite` may import `database/sql` but no concrete SQLite driver. The store receives an already-open `*sql.DB`, so driver selection remains a composition concern. Snapshot JSON is authoritative; a normalized `trend_metrics` table is a derived index for future trend queries. Migration/store SQL is validated against SQLite 3.46.1 through Python, while Go migration/codec/store behavior uses standard-library-only tests and a transaction-aware fake `database/sql/driver`.

**Tech Stack:** Go standard library (`database/sql`, `encoding/json`), SQLite SQL dialect validated against SQLite 3.46.1, existing `temporal.Store` contract.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Global Constraints

- No concrete SQLite driver import in `internal/platform/sqlite`.
- `database/sql` remains forbidden outside `internal/platform/sqlite` among common layers.
- Snapshot JSON remains generic and versioned; never add engine-specific persistence columns.
- Query-text sensitivity is filtered again at the SQLite codec/store boundary and must not be reintroduced.
- All dynamic SQL values use parameters; identifiers/schema are static source constants.
- Saves are idempotent only when an existing snapshot ID has the exact same canonical payload; a conflicting same-ID payload fails closed.
- Partial writes are forbidden: snapshot row and derived trend rows are one SQL transaction.
- Loaded payloads are cross-checked against indexed envelope columns before being returned.
- Unknown future `PRAGMA user_version` values fail closed rather than being downgraded.
- A concrete SQLite Go driver and live Go reopen tests remain environment-pending.

---

## Schema v1

```sql
CREATE TABLE snapshots (
    id TEXT PRIMARY KEY,
    target_fingerprint TEXT NOT NULL,
    engine TEXT NOT NULL,
    adapter_id TEXT NOT NULL,
    adapter_version TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    collected_at_ns INTEGER NOT NULL,
    payload_json BLOB NOT NULL
);

CREATE INDEX snapshots_target_time
    ON snapshots(target_fingerprint, collected_at_ns DESC, id DESC);

CREATE TABLE trend_metrics (
    snapshot_id TEXT NOT NULL,
    metric_kind TEXT NOT NULL CHECK(metric_kind IN ('observation', 'delta')),
    signal_key TEXT NOT NULL,
    object_kind TEXT NOT NULL,
    object_id TEXT NOT NULL,
    numeric_value REAL NOT NULL,
    unit TEXT NOT NULL,
    exactness TEXT NOT NULL,
    rate_per_second REAL,
    window_seconds REAL,
    FOREIGN KEY(snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE
);

CREATE INDEX trend_metrics_lookup
    ON trend_metrics(signal_key, object_kind, object_id, metric_kind, snapshot_id);
```

`metric_kind` fixes an ambiguity in the earlier temporal plan where observations and sampled deltas could not be distinguished after normalization. Delta rate/window fields are preserved rather than discarded.

---

### Task 1: Migration contract

**Files:**
- Create: `internal/platform/sqlite/migrations.go`
- Create: `internal/platform/sqlite/migrations_test.go`

- [x] Define schema version `1`, static DDL statements, `PRAGMA foreign_keys = ON`, and `PRAGMA busy_timeout = 5000`.
- [x] Add migration decision: version `0` => apply v1; version `1` => no-op; version `>1` => fail closed.
- [x] Execute DDL twice against SQLite 3.46.1 and verify `user_version`, tables, indexes, `metric_kind` CHECK and FK cascade.
- [x] Verify migration DDL + `PRAGMA user_version` commit and rollback semantics inside a real SQLite transaction.
- [x] Run dependency-free Go normal and race tests.

### Task 2: Snapshot codec and trend extraction

**Files:**
- Create: `internal/platform/sqlite/codec.go`
- Create: `internal/platform/sqlite/codec_test.go`

- [x] Encode/decode versioned snapshot JSON with `encoding/json`.
- [x] Revalidate through `temporal.NewSnapshot`; reject ID mismatch/corrupt schema version.
- [x] Reapply query-text sensitivity filtering at persistence boundary.
- [x] Extract numeric observations as `metric_kind=observation`.
- [x] Extract deltas as `metric_kind=delta`, preserving rate/window.
- [x] Keep deterministic trend row ordering.
- [x] Run dependency-free normal and race tests.

### Task 3: `database/sql` store adapter

**Files:**
- Create: `internal/platform/sqlite/baseline.go`
- Create: `internal/platform/sqlite/sql_contract_test.go`
- Create: `internal/platform/sqlite/fake_driver_test.go`

- [x] `New(ctx, db)` rejects nil, applies connection-local PRAGMAs and runs migration/bootstrap logic.
- [x] Each operation acquires its own `*sql.Conn` and reapplies connection-local `foreign_keys` / `busy_timeout` settings.
- [x] `Save` uses one transaction and writes trend rows only for a newly inserted snapshot.
- [x] Duplicate snapshot IDs are idempotent only when canonical payload bytes match; collisions fail closed.
- [x] Trend insert failure rolls back the snapshot insert.
- [x] `Latest`, `Previous`, and `List` read indexed envelope + payload and reject envelope/payload mismatch.
- [x] Missing history returns `temporal.ErrSnapshotNotFound`.
- [x] Standard-library fake driver models commit/rollback and query shapes; normal + race tests pass.
- [x] Execute equivalent insert/idempotency/trend/latest/previous/list/rollback/FK-cascade SQL against SQLite 3.46.1; passed.

### Task 4: Environment-dependent completion

- [ ] Select/pin a pure-Go SQLite driver only after current module metadata can be resolved.
- [ ] Add composition/opening code outside core/application contracts.
- [ ] Run live Go tests against a temporary on-disk SQLite database: migrate, save, duplicate save, conflicting duplicate rejection, close/reopen, latest/previous/list, foreign-key cascade and corrupted-payload failure.
- [ ] Wire persistent local history and `dbprobe diff` CLI only after live driver tests pass.
- [ ] Run the full Go 1.25 repository gate before declaring persistent baseline complete.

## Current status

```text
migration/schema contract        complete / Go race + SQLite 3.46.1 verified
snapshot codec/trend extraction  complete / Go race verified
database/sql Store               complete / fake-driver race + SQLite SQL verified
concrete pure-Go driver          pending dependency access
live Go close/reopen persistence pending dependency access
persistent CLI history/diff      pending live driver gate
```

The pre-driver phase is source-complete and verified within its stated boundary. It is **not** equivalent to live persistent-baseline acceptance; that final gate requires a concrete Go SQLite driver and process-level reopen tests.
