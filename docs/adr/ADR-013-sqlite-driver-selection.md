# ADR-013: SQLite Driver Selection

- **Status:** Accepted
- **Proposed:** 2026-08-21
- **Accepted:** 2026-09-02
- **Decision scope:** Local temporal baseline persistence

## Context

dbprobe needs an embedded SQLite database for local snapshot history, temporal diffs and normalized trend indexes. The default CLI distribution must remain cross-platform and CGo-free; requiring a C compiler or shipping a separately managed native SQLite library would complicate builds, cross-compilation and installation.

The persistence implementation in `internal/platform/sqlite` remains driver-independent. It consumes `database/sql` and exposes a connector boundary:

```go
type ConnectorFactory func(string) (driver.Connector, error)
```

`sqlite.Open(...)` owns the resulting `*sql.DB`, applies the one-connection pool policy, prepares every acquired connection, migrates the database and owns shutdown. Concrete-driver selection is therefore isolated from temporal domain and application code.

## Decision drivers

The production driver must satisfy all of the following:

1. CGo-free builds for the default distribution.
2. `database/sql` compatibility.
3. A `driver.Connector` path suitable for `sql.OpenDB`.
4. Compatibility with the repository's Go 1.25 baseline.
5. Linux, Windows and macOS support for the release workflow.
6. Active maintenance and a credible production adoption signal.
7. Correct transaction, foreign-key, PRAGMA, migration and on-disk reopen behavior under dbprobe tests.
8. No concrete driver import in `internal/core`, `internal/app`, `sdk`, surfaces or the driver-independent `internal/platform/sqlite` package.
9. Acceptable binary footprint and local-history latency for a CLI workload.

## Considered options

### `modernc.org/sqlite`

Accepted version: `v1.57.0`.

Advantages:

- CGo-free `database/sql` implementation.
- Stable v1 module with the stronger maturity and adoption signal of the CGo-free candidates considered.
- `NewConnector(dsn)` maps directly to dbprobe's `ConnectorFactory` and `sql.OpenDB` boundary.
- Module baseline is Go 1.25.
- Passed dbprobe's Linux, Windows and macOS CGo-free build matrix.
- Produced the smaller comparison binary.

Risks and costs:

- Larger transitive module graph than ncruces.
- The translated-runtime stack includes `modernc.org/libc`; ABI-compatible module versions must be retained.
- `v1.57.0` resolves `modernc.org/libc v1.74.4`; dbprobe must not independently force an incompatible libc version.
- It was slower than ncruces in the bounded comparison workload, although the absolute per-snapshot difference was below one millisecond.

### `github.com/ncruces/go-sqlite3`

Validated fallback version: `v0.35.3`.

Advantages:

- CGo-free `database/sql` implementation using the project's wasm-based SQLite runtime.
- `SQLite.OpenConnector(name)` satisfies the same dbprobe connector contract.
- Passed the same persistence contract and CGo-free cross-build matrix.
- Faster in the representative write/reopen microbenchmark.

Risks and costs:

- Pre-v1 module.
- Smaller maturity and adoption signal than modernc for the default production dependency.
- Produced a materially larger comparison binary despite its smaller conceptual direct-dependency surface.

### `github.com/mattn/go-sqlite3`

Rejected for the default distribution because it requires CGo and a C toolchain. It remains a mature external reference implementation, but its build and cross-compilation requirements conflict with the dbprobe CLI distribution contract.

## Decision

Use **`modernc.org/sqlite v1.57.0` as the default SQLite driver**.

Keep **`github.com/ncruces/go-sqlite3 v0.35.3` as a validated acceptance-only fallback**. It is present only in the isolated module under `test/acceptance/sqlite-drivers`; it is not part of the production module graph.

The concrete modernc import lives in the dedicated leaf package:

```text
internal/platform/sqlite/modernc
```

The generic `internal/platform/sqlite` package continues to depend only on `database/sql` and `database/sql/driver`. Architecture tests reject `modernc.org/sqlite` imports outside the dedicated binding leaf.

The default CLI composition supplies the modernc-backed owned-store factory. Consequently:

- `dbprobe inspect` persists snapshots to the platform data path by default;
- `dbprobe diff` is part of the default command graph;
- a history path-resolution or open failure does not block inspection and is rendered only as a generic history warning;
- `dbprobe diff` still fails when history is unavailable because it cannot produce a meaningful comparison without stored snapshots.

## Acceptance evidence

### Correctness

The live modernc path passed:

- create and migrate;
- save, close and reopen;
- `Latest`, `Previous` and `List`;
- duplicate snapshot idempotency;
- conflicting same-ID payload rejection;
- `PRAGMA foreign_keys = 1`;
- `PRAGMA busy_timeout = 5000`;
- `PRAGMA user_version = 1`;
- rollback after a forced trend-row insert failure;
- foreign-key cascade cleanup;
- private file permissions where supported;
- default CLI `inspect -> inspect -> diff` persistence;
- normal and race test suites.

### Toolchain and platforms

The production CLI passed CGo-free builds for:

- Linux amd64;
- Windows amd64;
- macOS amd64;
- macOS arm64.

Both modernc and ncruces comparison binaries passed the same CGo-free target matrix.

### Candidate comparison

Evidence is recorded in `docs/benchmarks/2026-09-02-sqlite-driver-selection.md` and was produced by workflow run `33646041100` on Go 1.25.14.

Median results over seven alternating runs with 250 snapshots, 32 observations and 16 deltas per snapshot:

| Metric | modernc v1.57.0 | ncruces v0.35.3 |
|---|---:|---:|
| Stripped comparison binary | 6.38 MiB | 8.70 MiB |
| Full process elapsed | 1,060.421 ms | 940.012 ms |
| Open + migrate | 2.519 ms | 2.522 ms |
| Write per snapshot | 4.007 ms | 3.541 ms |
| Reopen + read checks | 46.636 ms | 38.676 ms |

ncruces was approximately 13% faster overall. The absolute median write difference was approximately 0.47 ms per snapshot, while modernc produced a 2.32 MiB smaller comparison binary. Because target-database collection and counter sampling dominate a normal dbprobe inspection, the local persistence difference was judged non-material for the default CLI. Maturity, v1 stability and binary footprint therefore outweighed the microbenchmark advantage.

### MySQL integration

The modernc-backed default CLI also passed the Docker acceptance matrix against MySQL 8.0.46 and 8.4.11, including two persisted MySQL inspections followed by a versioned `dbprobe diff` report. The same gate retained adapter contract, schema fingerprint, plan sanitization and credential/privacy assertions.

## Consequences

- Persistent temporal history and `diff` are enabled in the default CLI.
- dbprobe release builds remain CGo-free.
- Concrete-driver replacement remains localized to one binding leaf and the composition root.
- The production module accepts modernc's larger transitive dependency graph and must respect its declared libc version.
- ncruces remains continuously reproducible as an acceptance-only fallback without increasing the production dependency graph.
- Driver upgrades must rerun the live reopen, full Go/race, CGo-free cross-build, candidate contract and representative comparison gates.

## Reconsideration triggers

Reopen this decision if any of the following occurs:

1. modernc/libc compatibility causes recurring build or runtime failures;
2. the modernc binary footprint becomes materially larger than the fallback;
3. local persistence becomes a significant portion of end-to-end inspection latency in production profiles;
4. a driver upgrade changes transaction, PRAGMA or close/reopen behavior;
5. either candidate materially changes its support or stability guarantees.
