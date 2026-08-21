# ADR-013: SQLite Driver Selection

- **Status:** Provisional
- **Date:** 2026-08-21
- **Decision scope:** Local temporal baseline persistence only

## Context

dbprobe needs an embedded SQLite database for local snapshot history, temporal diffs and trend indexes. The product targets a simple cross-platform CLI distribution, so requiring a C toolchain at build time or shipping a platform-specific native SQLite library is undesirable.

The persistence implementation in `internal/platform/sqlite` is deliberately driver-independent. It consumes `database/sql` and exposes a `ConnectorFactory` boundary:

```go
type ConnectorFactory func(string) (driver.Connector, error)
```

`sqlite.Open(...)` owns the resulting `*sql.DB`, configures a single-connection pool, applies/migrates the database through the generic store, and manages lifecycle. The concrete driver is therefore a composition-root decision rather than a persistence-domain decision.

## Decision drivers

A production driver must satisfy all of the following:

1. CGo-free builds for the default dbprobe distribution.
2. `database/sql` compatibility.
3. A `driver.Connector` path suitable for `sql.OpenDB`.
4. Compatibility with the repository's Go 1.25 baseline.
5. Linux, Windows and macOS support suitable for a single Go release workflow.
6. Active maintenance and a credible production user base.
7. Correct SQLite transaction, foreign-key, PRAGMA and on-disk reopen behavior under dbprobe's integration tests.
8. No concrete driver import inside `internal/core`, `internal/app`, `sdk`, or `internal/platform/sqlite`.

## Considered options

### `modernc.org/sqlite`

Verified candidate: `v1.56.0` (published 2026-08-03).

Advantages:

- CGo-free `database/sql` driver.
- Stable v1 module with broad adoption.
- `NewConnector(dsn)` was added in v1.56.0 specifically for `sql.OpenDB` and connection-interposition use cases.
- Go module declares `go 1.25.0`, matching dbprobe's minimum target.
- Supports the platforms relevant to the initial CLI distribution.

Risks / costs:

- Larger transitive dependency surface than the alternatives.
- The project explicitly documents `modernc.org/libc` as a fragile ABI dependency that must match the version declared by the SQLite module.
- For v1.56.0 the module declares `modernc.org/libc v1.74.4`; dbprobe must not independently upgrade that dependency.

### `github.com/ncruces/go-sqlite3`

Verified candidate: `v0.35.3` (published 2026-08-03).

Advantages:

- CGo-free SQLite implementation using wasm2go.
- `database/sql` driver.
- `SQLite.OpenConnector(name)` supports the connector boundary already implemented by dbprobe.
- Smaller direct dependency surface; project documentation describes Go and `x/sys` as its direct dependencies.

Risks / costs:

- Pre-v1 module.
- Significantly smaller adoption footprint than modernc.org/sqlite.
- The wasm2go architecture should be benchmarked for binary size, startup cost and representative dbprobe baseline workloads before preferring it over the more established modernc driver.

### `github.com/mattn/go-sqlite3`

Verified current v1 line candidate: `v1.14.49`.

Advantages:

- Very mature and widely deployed.
- `database/sql` compatible.

Rejected for the default distribution because:

- It requires CGo.
- It requires GCC/toolchain availability for builds.
- Cross-compilation and static-style release packaging become materially more complex, which conflicts with dbprobe's simple CLI distribution goal.

It may remain useful as a non-default compatibility/reference driver in external testing, but it is not the primary dbprobe choice.

## Decision

Use **`modernc.org/sqlite` as the provisional primary driver candidate** and keep **`github.com/ncruces/go-sqlite3` as the benchmark/fallback candidate**.

Do **not** add either dependency to `go.mod` until the environment can perform a real Go 1.25 module resolution and verification run.

If modernc v1.56.0 is selected by the live gate, its dependency graph must retain the module-declared `modernc.org/libc v1.74.4` compatibility; dbprobe must not hand-author a mismatched libc pin or `go.sum`.

The production driver import will live at the composition root (for example `cmd/dbprobe` or an equally narrow wiring package). `internal/platform/sqlite` will continue to accept only a `driver.Connector` factory and will not import either concrete driver.

## Live selection gate

Before changing this ADR to **Accepted**, run both CGo-free candidates through the same test matrix where practical:

1. Go 1.25.13 build/test on Linux amd64.
2. Windows amd64 and macOS cross-build smoke checks.
3. Create database → migrate → save snapshots → close process handle → reopen → `Latest` / `Previous` / `List`.
4. Duplicate snapshot idempotency and conflicting-payload rejection.
5. Foreign-key cascade and transaction rollback behavior.
6. Connection-local PRAGMA behavior.
7. Representative snapshot write/read latency.
8. Final binary-size comparison.
9. `go mod tidy` stability and dependency audit.

Choose ncruces instead if it materially improves release simplicity/binary footprint without failing correctness, compatibility or performance gates.

## Consequences

- dbprobe's temporal store is no longer structurally blocked on a specific SQLite package.
- Driver replacement remains a composition-only change.
- The current integration branch can continue validating persistence behavior with the standard library fake driver and SQLite SQL engine while dependency access is unavailable.
- Persistent CLI history is still not considered production-accepted until one concrete driver passes the live reopen matrix.
