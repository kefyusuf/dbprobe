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
- It was slower than ncruces in the bounded comparison workload, although the corrected normal-write difference was approximately 0.076 ms per snapshot.

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
- history path-resolution, open or close failure does not block a completed inspection and is rendered only as a generic history warning;
- underlying filesystem and driver details are not copied into the report;
- the target inspection is never rerun to recover from a persistence failure;
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
- stateless inspect fallback on history path/open failure;
- successful report rendering with a generic warning on history close failure;
- normal and race test suites.

### Toolchain and platforms

The production CLI passed CGo-free builds for:

- Linux amd64;
- Windows amd64;
- macOS amd64;
- macOS arm64.

Both modernc and ncruces comparison binaries passed the same CGo-free target matrix.

### Candidate comparison

Corrected evidence is recorded in `docs/benchmarks/2026-09-02-sqlite-driver-selection.md` and was produced by workflow run `33647790603` on Go 1.25.14. The artifact is `sqlite-driver-comparison`, ID `9853550603`.

The corrected benchmark separates normal writes from duplicate validation and separates reopen/read work from conflicting-payload and close checks. Median results over seven alternating runs with 250 snapshots, 32 observations and 16 deltas per snapshot:

| Metric | modernc v1.57.0 | ncruces v0.35.3 |
|---|---:|---:|
| Stripped comparison binary | 6.383 MiB | 8.699 MiB |
| Full process elapsed | 850.364 ms | 816.795 ms |
| Open + migrate | 2.636 ms | 2.574 ms |
| 250 normal snapshot writes | 793.949 ms | 774.863 ms |
| Write per normal snapshot | 3.176 ms | 3.099 ms |
| Reopen + read checks | 34.336 ms | 30.556 ms |

ncruces was approximately 2.5% faster for ordinary snapshot writes and 4.1% faster for the full process. The absolute median write difference was approximately 0.076 ms per persisted snapshot, while modernc produced a 2.316 MiB smaller comparison binary. Because target-database collection and counter sampling dominate a normal dbprobe inspection, the local persistence difference was judged non-material for the default CLI. Maturity, v1 stability and binary footprint therefore outweighed the microbenchmark advantage.

### MySQL integration

The modernc-backed default CLI passed the Docker acceptance matrix against MySQL 8.0.46 and 8.4.11 on the same corrected acceptance revision, including two persisted MySQL inspections followed by a versioned `dbprobe diff` report. The gate retained adapter contract, schema fingerprint, plan sanitization and credential/privacy assertions.

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
