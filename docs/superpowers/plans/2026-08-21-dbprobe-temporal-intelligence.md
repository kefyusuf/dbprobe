# dbprobe Temporal Intelligence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add database-agnostic snapshots, cross-inspection diffs, regression events, and local baseline persistence without introducing engine-specific semantics into core.

**Architecture:** Temporal analysis consumes the same capability/signal/finding contracts produced by every adapter. The core snapshot/diff/event packages have no database driver dependency. Persistence is behind a `Store` contract; an in-memory store proves behavior first, then a generic SQLite store persists versioned snapshot payloads and indexed numeric trend metrics.

**Tech Stack:** Go 1.25 standard library for core temporal logic; SQLite behind `database/sql` in `internal/platform/sqlite` only. The SQLite driver/version is selected only when the persistence task begins and can be verified in an environment with dependency access.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Global Constraints

- Temporal core must not import a concrete database adapter or engine driver.
- `internal/core`, `internal/app`, `internal/surfaces`, and `sdk` remain forbidden from importing `database/sql`.
- `internal/platform/sqlite` is the only common-layer path allowed to use `database/sql`; it must not import MySQL/PostgreSQL/MongoDB/Cassandra drivers.
- Snapshot storage is generic and must not add engine-specific columns when a new adapter is introduced.
- Counter resets must be represented explicitly and never interpreted as negative regression.
- Cross-inspection query regression is based on bounded sampled deltas, not lifetime cumulative totals.
- Snapshot/report payloads must preserve exactness and sensitivity metadata.
- Sensitive values classified above metadata/query-shape must not be newly persisted by the temporal layer.
- Snapshot schema is versioned independently from inspect JSON schema.
- Tasks 1-5 are dependency-free and must be implementable/testable without SQLite.
- SQLite persistence and CLI history wiring are not considered verified until Go 1.25 dependency resolution and tests can run.

---

## Locked Package Structure

```text
internal/
├── core/
│   └── temporal/
│       ├── snapshot.go
│       ├── diff.go
│       ├── query_regression.go
│       ├── events.go
│       └── *_test.go
├── app/
│   └── diff/
│       ├── service.go
│       └── service_test.go
├── platform/
│   ├── baseline/
│   │   ├── store.go
│   │   ├── memory.go
│   │   └── memory_test.go
│   └── sqlite/
│       ├── baseline.go
│       ├── migrations.go
│       └── baseline_test.go
└── surfaces/
    └── terminal/
        └── diff.go
```

`internal/core/temporal` is pure domain/runtime logic. `internal/platform/baseline` owns the persistence port plus a deterministic memory implementation for tests. `internal/platform/sqlite` is an infrastructure adapter for that port.

---

### Task 1 — Snapshot Contract and Architecture Guard Exception

**Files:**
- Modify: `test/architecture/dependencies_test.go`
- Create: `internal/core/temporal/snapshot.go`
- Create: `internal/core/temporal/snapshot_test.go`

**Produces:**

```go
const SnapshotSchemaVersion = "dbprobe.snapshot/v1alpha1"

type Snapshot struct {
    SchemaVersion     string
    ID                string
    TargetFingerprint string
    Engine            string
    AdapterID         string
    AdapterVersion    string
    CollectedAt       time.Time
    Capabilities      []capability.Capability
    Observations      []signal.Observation
    Deltas            []signal.Delta
    Findings          []finding.Finding
}
```

- [ ] Add an architecture-test rule that continues forbidding engine drivers everywhere outside adapters/composition root, while permitting `database/sql` only under `internal/platform/sqlite`.
- [ ] Write snapshot tests asserting deterministic ID for identical target/time input, non-empty schema version, sorted capabilities, and defensive copies of slices.
- [ ] Snapshot ID seed is `targetFingerprint|collectedAt.UTC().Format(time.RFC3339Nano)` hashed with SHA-256 and encoded from the first 16 bytes.
- [ ] `NewSnapshot` rejects empty target fingerprint, engine, adapter ID, or zero collection time.
- [ ] Snapshot construction must not inspect engine names or signal namespaces.
- [ ] Run `go test ./internal/core/temporal ./test/architecture -v` and commit `feat: define temporal snapshot contract`.

---

### Task 2 — Cross-Snapshot Observation Diff

**Files:**
- Create: `internal/core/temporal/diff.go`
- Create: `internal/core/temporal/diff_test.go`

**Produces:**

```go
type ChangeKind string
const (
    ChangeAdded   ChangeKind = "added"
    ChangeRemoved ChangeKind = "removed"
    ChangeChanged ChangeKind = "changed"
    ChangeReset   ChangeKind = "reset"
)

type Change struct {
    Kind       ChangeKind
    Key        signal.Key
    Object     object.Ref
    Before     *signal.Observation
    After      *signal.Observation
    NumericDelta *float64
}

func Compare(previous, current Snapshot) (Diff, error)
```

- [ ] Write tests for numeric gauge change, object added/removed, unchanged evidence omission, and deterministic output ordering.
- [ ] Observation identity is `signal key + object kind + object id`; engine-specific object kinds remain opaque strings.
- [ ] For numeric cumulative observations: current < previous produces `reset`, never a negative numeric change.
- [ ] For scraped/estimated/sample observations: numeric delta is `after-before` when both are numeric.
- [ ] Type changes (`number→text`, etc.) are represented as `changed` without inventing a numeric delta.
- [ ] Compare rejects different target fingerprints.
- [ ] Run tests and commit `feat: add reset-aware snapshot diff`.

---

### Task 3 — Query Regression Derivation

**Files:**
- Create: `internal/core/temporal/query_regression.go`
- Create: `internal/core/temporal/query_regression_test.go`

**Produces:**

```go
type QueryRegression struct {
    Object                object.Ref
    PreviousMeanLatencyMS float64
    CurrentMeanLatencyMS  float64
    Ratio                 float64
    AddedLatencyMS        float64
    CurrentCalls          float64
}

func DetectQueryRegressions(previous, current Snapshot, policy QueryRegressionPolicy) []QueryRegression
```

Default policy:

```text
minimum current calls: 10
minimum previous calls: 5
minimum absolute latency increase: 5ms
minimum ratio: 2.0x
```

- [ ] Derive mean latency from sampled-window deltas: `mysql.query.total_latency_ms / core.query.calls` for the same query object. The temporal package may key on portable `core.query.calls` plus a latency signal supplied by an adapter; the default MVP policy registers MySQL's latency key from the application layer, not through `if engine` logic inside the generic detector.
- [ ] To avoid engine branching, implement a generic `MetricPair{CallsKey, TotalLatencyKey}` parameter and have MySQL callers pass their keys.
- [ ] Ignore lifetime cumulative observations completely.
- [ ] Ignore reset/zero/insufficient-volume windows.
- [ ] `AddedLatencyMS = currentCalls * max(currentMean-previousMean, 0)` within the current sample window.
- [ ] Sort regressions by added latency descending, then object identity.
- [ ] Run tests and commit `feat: detect sampled query regressions`.

---

### Task 4 — Temporal Events

**Files:**
- Create: `internal/core/temporal/events.go`
- Create: `internal/core/temporal/events_test.go`

**Produces:**

```go
type EventType string
const (
    EventSignalReset      EventType = "signal_reset"
    EventObjectAppeared   EventType = "object_appeared"
    EventObjectDisappeared EventType = "object_disappeared"
    EventQueryRegression  EventType = "query_regression"
)

type Event struct {
    Type       EventType
    Object     object.Ref
    SignalKey  signal.Key
    Summary    string
    CollectedAt time.Time
}
```

- [ ] Convert diff reset/add/remove changes into deterministic events.
- [ ] Convert query regressions into `query_regression` events without copying raw SQL/query literals.
- [ ] Event summaries may contain signal keys, ratios, numeric metrics and object IDs only.
- [ ] Sort by event type/object/signal key for deterministic JSON/storage behavior.
- [ ] Run tests and commit `feat: derive temporal events`.

---

### Task 5 — Baseline Store Contract, Memory Store and Diff Service

**Files:**
- Create: `internal/platform/baseline/store.go`
- Create: `internal/platform/baseline/memory.go`
- Create: `internal/platform/baseline/memory_test.go`
- Create: `internal/app/diff/service.go`
- Create: `internal/app/diff/service_test.go`

**Store:**

```go
type Store interface {
    Save(context.Context, temporal.Snapshot) error
    Latest(context.Context, string) (*temporal.Snapshot, error)
    Previous(context.Context, string, time.Time) (*temporal.Snapshot, error)
    List(context.Context, string, int) ([]temporal.Snapshot, error)
}
```

- [ ] Memory store enforces target fingerprint partitioning and collection-time ordering.
- [ ] Saving the same snapshot ID twice is idempotent.
- [ ] `Previous` returns the newest snapshot strictly earlier than the supplied time.
- [ ] Store returns defensive snapshot copies.
- [ ] Diff application service loads the latest two snapshots for one target, invokes temporal diff and optional query-regression derivation, then returns a versioned `dbprobe.diff/v1alpha1` report.
- [ ] No adapter import is allowed in the diff service.
- [ ] Run `go test ./internal/platform/baseline ./internal/app/diff -v` and commit `feat: add baseline store contract and diff service`.

---

### Task 6 — Generic SQLite Baseline Persistence

**Files:**
- Create: `internal/platform/sqlite/migrations.go`
- Create: `internal/platform/sqlite/baseline.go`
- Create: `internal/platform/sqlite/baseline_test.go`
- Modify: `go.mod`, `go.sum`

**Schema:**

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
    ON snapshots(target_fingerprint, collected_at_ns DESC);

CREATE TABLE trend_metrics (
    snapshot_id TEXT NOT NULL,
    signal_key TEXT NOT NULL,
    object_kind TEXT NOT NULL,
    object_id TEXT NOT NULL,
    numeric_value REAL NOT NULL,
    unit TEXT NOT NULL,
    exactness TEXT NOT NULL,
    FOREIGN KEY(snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE
);
CREATE INDEX trend_metrics_lookup
    ON trend_metrics(signal_key, object_kind, object_id, snapshot_id);
```

- [ ] Select a pure-Go SQLite driver only when current package metadata can be verified; do not guess a dependency version.
- [ ] Migration uses `PRAGMA user_version` and is idempotent.
- [ ] Persist the complete versioned snapshot as JSON plus numeric observations/deltas in `trend_metrics` for future trend queries.
- [ ] Use prepared/parameterized SQL only.
- [ ] Enable foreign keys and a bounded busy timeout; do not use engine-specific columns.
- [ ] Tests use a temporary on-disk database, verify save/reopen/latest/previous/idempotency, and ensure JSON round-trip preserves exactness/sensitivity metadata.
- [ ] This task remains pending if dependency resolution cannot be verified in the current environment.

---

### Task 7 — Inspect History Wiring and CLI Diff

**Files:**
- Modify: `internal/app/inspect/service.go`
- Add: `internal/app/inspect/history.go`
- Modify: `cmd/dbprobe/root.go`
- Create: `cmd/dbprobe/diff.go`
- Create: `internal/surfaces/terminal/diff.go`
- Add tests.

- [ ] Preserve existing `inspect.New(registry, planner)` API; add history through an explicit option/decorator so foundation callers do not break.
- [ ] When history is enabled, a successful inspection saves a snapshot only after collection/finding evaluation completes.
- [ ] Persistence failure must not silently change diagnostic evidence; return a distinct history error or warning according to the finalized application policy.
- [ ] `dbprobe diff <target>` resolves the target fingerprint without running unsafe statements, loads two latest snapshots, and renders `dbprobe.diff/v1alpha1`.
- [ ] Default local database path is platform-appropriate under the user's config/data directory; a CLI flag can override it.
- [ ] No SQLite code leaks into core/application contracts.
- [ ] This task starts only after Task 6 passes.

---

## Acceptance Gate

Dependency-free temporal core is considered complete when:

```bash
go test ./internal/core/temporal ./internal/platform/baseline ./internal/app/diff -v
go test -race ./internal/core/temporal ./internal/platform/baseline ./internal/app/diff
```

Full temporal MVP additionally requires in an environment with Go 1.25 and dependency access:

```bash
go mod tidy
gofmt -w ...
go vet ./...
go test ./...
go test -race ./...
```

and SQLite reopen/persistence tests.

## Definition of Done

Two inspections of the same target can be stored generically, compared without engine-specific branches in core, counter resets are distinguished from regressions, sampled query latency regressions become deterministic temporal events, and the latest/previous snapshots survive process restarts through a local SQLite baseline store.
