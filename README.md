# dbprobe

Database intelligence runtime for deterministic, read-only diagnostics, temporal analysis, CI/agent surfaces, and database-specific intelligence behind modular adapters.

> **v0.1 integration candidate:** bounded collection, generic core findings, temporal intelligence, persistent SQLite history, the MySQL 8.0/8.4 adapter, structural schema fingerprinting, safe plan-only EXPLAIN, and the test-only MongoDB semantic probe are integrated. Exact squash-merged revision `84d9f7dcec88b206864cef71a90a2922d408bcbd` on `integration-v0.1` passed permanent CI run `33651852902`, including the complete Go 1.25 normal/race/build/smoke gate and CGo-free Linux/Windows/macOS builds. Revision-bound acceptance also covers live SQLite close/reopen behavior and the MySQL 8.0.46/8.4.11 Docker matrix. Promotion toward `main` remains a separate review gate; this is not a tagged release.

## Architecture

`dbprobe` keeps database-specific mechanics out of the shared runtime.

```text
CLI / JSON / future MCP / AI explanation
                  |
            Application
     inspect / diff / explain
                  |
 Core: collection / findings / temporal
                  |
              Adapter SDK
          /         |          \
       MySQL     Mongo probe   future adapters

Local temporal persistence
  internal/platform/sqlite            driver-independent store
  internal/platform/sqlite/modernc    concrete CGo-free binding
```

The shared core consumes capabilities, observations, deltas, findings, snapshots and events. It does not branch on MySQL/PostgreSQL/MongoDB/Cassandra engine names.

The adapter SDK intentionally remains **v0.1**. The MongoDB semantic probe demonstrates that current contracts are not relational-only, but a real non-relational adapter and later distributed-database validation are still required before an Adapter Contract v1.0 promise.

## Integrated capabilities

### Shared runtime

- capability-driven collectors;
- bounded concurrent collection, default concurrency `4`;
- centralized counter double-sampling and reset-safe deltas;
- deterministic core and adapter-native findings;
- authoritative generic `core.*` rules without adapter-local duplicates;
- versioned inspect JSON: `dbprobe.inspect/v1alpha1`;
- generic temporal snapshots, diffs, query regressions and bounded-time events;
- in-memory and SQLite-backed temporal stores;
- query-text persistence filtering.

### SQLite temporal history

The driver-independent implementation under `internal/platform/sqlite` provides:

- versioned migrations through `PRAGMA user_version`;
- `foreign_keys=ON` and a 5-second busy timeout on every acquired connection;
- atomic snapshot and derived trend-row transactions;
- versioned JSON snapshot payloads and normalized observation/delta indexes;
- explicit `observation` and `delta` metric kinds, including sampled rate/window semantics;
- snapshot identity-collision and envelope/payload corruption checks;
- query-text sensitivity re-filtering at the persistence boundary;
- a 16 MiB serialized snapshot limit and 100,000 trend-row limit per snapshot;
- a one-connection pool, owned lifecycle and idempotent close;
- private data-directory defaults and best-effort `0600` file permissions where supported.

The default binding is **`modernc.org/sqlite v1.57.0`**. Its import is isolated to `internal/platform/sqlite/modernc`; architecture tests reject concrete-driver imports elsewhere in shared code. `github.com/ncruces/go-sqlite3 v0.35.3` remains a validated fallback in the isolated acceptance module and is not part of the production dependency graph.

The live modernc gate verifies:

- create, migrate, save, close and reopen;
- `Latest`, `Previous` and `List`;
- duplicate idempotency and conflicting-payload rejection;
- connection-local PRAGMAs;
- transaction rollback and foreign-key cascade;
- default CLI `inspect -> inspect -> diff` persistence;
- CGo-free Linux, Windows and macOS builds.

The corrected candidate comparison and decision rationale are recorded in `docs/benchmarks/2026-09-02-sqlite-driver-selection.md` and `docs/adr/ADR-013-sqlite-driver-selection.md`.

### History availability semantics

`dbprobe inspect` treats local history as optional operational infrastructure:

- when the history path resolves and SQLite opens, the inspection snapshot is persisted;
- path-resolution or store-open failure falls back to a stateless inspection;
- snapshot construction/save failure remains a non-fatal `history` warning from the application service;
- store-close failure does not replace an otherwise successful inspection result;
- all local-history failures use a generic warning and do not copy filesystem, path or driver details into the report;
- target collection is never repeated to recover from a persistence failure;
- `dbprobe diff` remains strict and returns an error when history is unavailable because no meaningful comparison can be produced.

### MySQL adapter

The primary production target is MySQL **8.4 LTS**. MySQL **8.0.46** remains in the acceptance matrix as legacy compatibility coverage.

The adapter includes:

- bounded connection runtime and stable target fingerprinting;
- Performance Schema and Information Schema capability discovery;
- connection, query digest, index, table, transaction, lock, replication, buffer-pool and InnoDB telemetry;
- long-transaction, lock-wait, blocking-chain, query amplification/full-scan, index, replication, buffer-pool, deadlock, redo/temp-table and purge/history-list findings;
- active replication apply-lag measurement that excludes intentional replication delay;
- structural schema fingerprinting;
- safe plan-only MySQL EXPLAIN.

The MySQL 8.0.46/8.4.11 Docker gate verifies adapter contracts, capability truthfulness, schema fingerprint stability, sanitized plans, credential/privacy behavior and two persisted MySQL inspections followed by `dbprobe diff`.

### Structural schema fingerprint

The MySQL adapter emits only an opaque versioned digest:

```text
mysql.schema.structural_fingerprint
v1:sha256:<64 lowercase hex>
```

Fingerprint v1 covers tables, columns, indexes, key/constraint relationships, CHECK clauses and foreign-key referential actions. Raw defaults, generated expressions, CHECK clauses and referential definitions are hash inputs only; they are not emitted as observations.

Resource bounds are fail-closed: 1 MiB per metadata field, 4 MiB per canonical record, 100,000 rows per metadata group and 64 MiB total canonical metadata.

## CLI

### Inspect

```bash
dbprobe inspect fake://local
```

```bash
dbprobe inspect 'mysql://dbprobe:password@127.0.0.1:3306/shop' \
  --format=json \
  --sample-window=1s
```

When local history is available, successful inspections are persisted automatically to the platform data file. When it is unavailable, `inspect` still returns its diagnostic report with a generic history warning and does not claim that the snapshot was stored.

| Platform | Default location |
|---|---|
| Linux | `$XDG_DATA_HOME/dbprobe/dbprobe.db`, or `~/.local/share/dbprobe/dbprobe.db` |
| macOS | `~/Library/Application Support/dbprobe/dbprobe.db` |
| Windows | `%LOCALAPPDATA%\dbprobe\dbprobe.db` |

### Diff

After at least two persisted inspections of the same target:

```bash
dbprobe diff 'mysql://dbprobe:password@127.0.0.1:3306/shop'
```

```bash
dbprobe diff 'mysql://dbprobe:password@127.0.0.1:3306/shop' --format=json
```

The JSON contract is versioned as `dbprobe.diff/v1alpha1`.

### Explain

```bash
dbprobe explain 'mysql://dbprobe:password@127.0.0.1:3306/shop' \
  --statement 'SELECT * FROM shop.orders WHERE customer_id = 1' \
  --format=json
```

`dbprobe explain` does **not** use `EXPLAIN ANALYZE`. It accepts one conservative `SELECT`, runs `EXPLAIN FORMAT=JSON` inside a bounded read-only transaction, rolls it back and sanitizes the JSON plan before rendering. Literal conditions and unknown scalar plan fields are not emitted.

MySQL URI options are restricted to diagnostic connection settings such as `tls`, `timeout`, `readTimeout` and `writeTimeout`. Options that expand driver behavior, including multi-statements and local-file access, are rejected.

## Safety and privacy

- Deterministic code owns diagnosis; AI is explanatory only.
- Diagnostic collectors are read-only and do not perform remediation.
- Credentials are never emitted or persisted.
- Query workload evidence uses normalized digest/query-shape representations.
- Transaction query text is not collected.
- Replication error-message text is not collected because it can contain application values.
- Raw query statements are excluded from explain-report serialization.
- Raw MySQL EXPLAIN JSON is sanitized before crossing the adapter boundary.
- Missing privileges reduce advertised capabilities instead of pretending visibility is complete.
- Temporal persistence filters raw query-text sensitivity classes.
- SQLite snapshot persistence is size-bounded and fails closed on identity/envelope corruption.
- Local-history path, open, save and close failures are represented by generic warnings without leaking filesystem or driver details.

## Non-relational architecture probe

`test/architecture/mongodb_probe_test.go` is a test-only MongoDB semantic adapter. It uses MongoDB-native capabilities, object kinds and signals (`mongodb.wiredtiger`, `mongodb.query_shape`, replica-set/oplog concepts) through the same collection, finding and temporal pipelines. It is architecture evidence, not production MongoDB support.

## Development

### Complete Go gate

```bash
make ci
```

This runs:

```text
go mod tidy + module diff check
gofmt check
go vet ./...
go test ./...
go test -race ./...
CGo-free production build
Linux/Windows/macOS cross-builds
SQLite candidate persistence contracts
benchmark input-validation regression
persistent inspect/diff CLI smoke tests
privacy and invalid-input smoke assertions
```

### SQLite candidate contracts and comparison

```bash
make test-sqlite-drivers
make cross-build-sqlite-drivers
RUNS=7 SNAPSHOTS=250 make compare-sqlite-drivers
```

The ncruces comparison dependency is confined to `test/acceptance/sqlite-drivers`, which is a separate Go module. Benchmark timing reports normal writes, duplicate validation, close, reopen/read and conflicting-payload checks as separate phases. Run and snapshot counts must use canonical positive decimal notation; snapshot count must be at least two.

### MySQL acceptance

```bash
make test-mysql
```

This starts pinned MySQL 8.0.46 and 8.4.11 containers, runs integration and adapter-contract tests, verifies sanitized EXPLAIN output and exercises persistent MySQL history plus `diff` through the production CGo-free binary.

See `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`, `docs/adr/ADR-013-sqlite-driver-selection.md`, `docs/benchmarks/2026-09-02-sqlite-driver-selection.md`, and the implementation plans under `docs/superpowers/plans/` for the locked architecture, evidence and execution history.
