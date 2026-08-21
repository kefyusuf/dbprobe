# dbprobe SQLite Baseline Pre-Driver Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement and verify the driver-independent SQLite persistence boundary—schema, migrations, snapshot codec, SQL store behavior contracts, and trend-row extraction—while deferring only concrete Go SQLite driver registration/live reopen tests until dependency access is available.

**Architecture:** `internal/platform/sqlite` may import `database/sql` but no concrete SQLite driver. The store receives an already-open `*sql.DB`, so driver selection remains a composition concern. Snapshot JSON is authoritative; a normalized `trend_metrics` table is a derived index for future trend queries. Migration SQL is validated against the environment's SQLite 3.46.1 through Python, while Go codec/migration-decision logic remains standard-library testable.

**Tech Stack:** Go standard library (`database/sql`, `encoding/json`), SQLite SQL dialect validated against SQLite 3.46.1, existing `temporal.Store` contract.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Global Constraints

- No concrete SQLite driver import in `internal/platform/sqlite`.
- `database/sql` remains forbidden outside `internal/platform/sqlite` among common layers.
- Snapshot JSON remains generic and versioned; never add engine-specific persistence columns.
- Query-text sensitivity is already filtered before snapshots reach the store and must not be reintroduced.
- All dynamic SQL values use parameters; identifiers/schema are static source constants.
- Saves are idempotent by snapshot ID.
- Partial writes are forbidden: snapshot row and derived trend rows are one SQL transaction.
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

- [ ] Define schema version `1`, static DDL statements, `PRAGMA foreign_keys = ON`, and `PRAGMA busy_timeout = 5000`.
- [ ] Add a pure migration-decision helper: version `0` => apply v1; version `1` => no-op; version `>1` => error.
- [ ] Verify DDL by executing it twice against a temporary Python SQLite 3.46.1 database and checking `PRAGMA user_version = 1` plus expected tables/indexes/columns.

### Task 2: Snapshot codec and trend extraction

**Files:**
- Create: `internal/platform/sqlite/codec.go`
- Create: `internal/platform/sqlite/codec_test.go`

- [ ] Encode snapshot JSON with `encoding/json`.
- [ ] Decode and revalidate through `temporal.NewSnapshot`; reject ID mismatch/corrupt payload.
- [ ] Extract numeric observations as `metric_kind=observation`.
- [ ] Extract sampled/numeric deltas as `metric_kind=delta`, preserving rate/window.
- [ ] Keep deterministic row ordering for repeatable tests.
- [ ] Run normal and race tests dependency-free.

### Task 3: `database/sql` store adapter

**Files:**
- Create: `internal/platform/sqlite/baseline.go`
- Create: `internal/platform/sqlite/sql_contract_test.go`

- [ ] `New(db *sql.DB)` rejects nil and runs migration/bootstrap logic.
- [ ] `Save` uses one transaction, `INSERT OR IGNORE` for snapshot idempotency, and inserts trend rows only for a newly inserted snapshot.
- [ ] `Latest`, `Previous`, and `List` load only payload JSON then use the codec for validation/defensive copies.
- [ ] SQL contract tests use a small standard-library test driver to assert transaction boundaries, parameterization, idempotent insert decision, and query shapes. These tests do not claim SQLite engine semantics.

### Task 4: Environment-dependent completion

- [ ] Select/pin a pure-Go SQLite driver only after current module metadata can be resolved.
- [ ] Add composition/opening code outside core.
- [ ] Run live Go tests against a temporary on-disk SQLite database: migrate, save, duplicate save, close/reopen, latest/previous/list, foreign-key cascade and corrupted-payload failure.
- [ ] Run the full Go 1.25 repository gate before declaring persistent baseline complete.

## Acceptance for this pre-driver phase

The phase is complete when migration DDL is proven valid on SQLite 3.46.1, codec/trend extraction passes Go normal + race tests, and the `database/sql` store contract is implemented without importing a concrete driver. It is **not** equivalent to live persistent-baseline acceptance; that remains blocked on Go SQLite driver dependency access.
