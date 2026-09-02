# dbprobe

Database intelligence runtime for deterministic, read-only diagnostics, temporal analysis, CI/agent surfaces, and database-specific intelligence behind modular adapters.

> **v0.1 integration status:** source integration continues on `integration-v0.1`. Foundation, bounded collection concurrency, generic core findings, temporal intelligence, driver-independent SQLite baseline persistence, driver-independent history-aware CLI orchestration, the MySQL MVP adapter, MySQL schema fingerprinting, safe plan-only EXPLAIN, and a test-only MongoDB semantic probe are integrated. The current pre-driver tree passes the full Go 1.25 `make ci` gate and Cobra binary smoke tests. The principal remaining release gates are the concrete `modernc.org/sqlite` binding with live on-disk close/reopen verification, a final driver-inclusive Go 1.25 gate, and Docker acceptance against MySQL 8.0.46 and 8.4.11.

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
```

The shared core consumes capabilities, observations, deltas, findings, snapshots, and events. It does not branch on MySQL/PostgreSQL/MongoDB/Cassandra engine names.

The adapter SDK intentionally remains **v0.1**. The MongoDB semantic probe demonstrates that current contracts are not relational-only, but a real non-relational adapter and later distributed-database validation are still required before an Adapter Contract v1.0 promise.

## Integrated capabilities

### Shared runtime

- capability-driven collectors;
- bounded concurrent collection, default concurrency `4`;
- centralized counter double-sampling and reset-safe deltas;
- deterministic core + adapter-native findings;
- core rule IDs take precedence over accidental adapter duplicates;
- versioned inspect JSON: `dbprobe.inspect/v1alpha1`;
- generic temporal snapshots, diffs, query regressions and bounded-time events;
- in-memory and SQLite-backed baseline implementations behind the generic temporal store port;
- query-text persistence filtering.

### SQLite temporal baseline

`internal/platform/sqlite` is a driver-independent `database/sql` persistence adapter. It does not import a concrete SQLite implementation.

Current source-level persistence includes:

- versioned SQLite migration schema with `PRAGMA user_version`;
- `foreign_keys=ON` and bounded busy timeout on every acquired connection;
- atomic snapshot + derived trend-row transactions;
- versioned JSON snapshot payloads plus normalized observation/delta trend indexes;
- explicit `observation` vs `delta` metric kinds, preserving sampled rate/window data;
- snapshot identity-collision and envelope/payload corruption checks;
- query-text sensitivity re-filtering at the persistence boundary;
- 16 MiB serialized snapshot limit and 100,000 derived trend-row limit per snapshot;
- owned-store lifecycle with a single-connection pool and idempotent close;
- private data-directory defaults (`XDG_DATA_HOME`/macOS Application Support/Windows LOCALAPPDATA) and the `dbprobe/dbprobe.db` namespace;
- best-effort private baseline-file permissions (`0600`) on supported platforms.

The concrete-driver-independent CLI layer is also complete:

- `inspect` can receive an owned history-store factory and persist snapshots through the existing application service;
- `diff` resolves the target through the adapter registry, reads the two latest compatible snapshots, and renders the versioned diff contract;
- both commands validate their arguments before opening local history;
- every opened history store is closed by the command orchestration boundary;
- CLI and platform persistence use the same `datadir.BaselineDBPath()` source of truth;
- the default driverless binary remains stateless and does not register `diff`, preventing a command that cannot yet open a real SQLite database from being advertised.

The SQL migration/store behavior has been exercised against SQLite 3.46.1. The application-level `inspect -> history -> diff` flow passes normal and race tests, and the complete pre-driver repository passes Go 1.25 module, formatting, vet, normal-test, race-test, build, and CLI smoke gates.

Concrete driver selection is intentionally isolated at the composition root. `docs/adr/ADR-013-sqlite-driver-selection.md` records `modernc.org/sqlite` as the provisional primary candidate and `github.com/ncruces/go-sqlite3` as the benchmark/fallback candidate. The next SQLite step is to pin the selected driver, provide the production history-store factory, run live on-disk close/reopen tests, and only then enable persistent history and `diff` in the default command graph.

### MySQL adapter

Primary production target is MySQL **8.4 LTS**. MySQL **8.0.46** remains in the integration matrix as legacy compatibility coverage.

The adapter currently includes:

- bounded connection runtime and stable target fingerprinting;
- Performance Schema / Information Schema capability discovery;
- connection, query digest, index, table, transaction, lock, replication, buffer-pool and InnoDB telemetry;
- long-transaction, lock-wait, blocking-chain, query amplification/full-scan, index, replication, buffer-pool, deadlock, redo/temp-table and purge/history-list findings;
- active replication apply-lag measurement that excludes intentional replication delay;
- structural schema fingerprinting;
- safe plan-only MySQL EXPLAIN.

### Structural schema fingerprint

The MySQL adapter emits only an opaque versioned digest:

```text
mysql.schema.structural_fingerprint
v1:sha256:<64 lowercase hex>
```

Fingerprint v1 covers tables, columns, indexes, key/constraint relationships, CHECK clauses and foreign-key referential actions. Raw defaults, generated expressions, CHECK clauses and referential definitions are hash inputs only; they are not emitted as observations.

Resource bounds are fail-closed: 1 MiB per metadata field, 4 MiB per canonical record, 100,000 rows per metadata group, and 64 MiB total canonical metadata.

## CLI

Inspect a fake target:

```bash
dbprobe inspect fake://local
```

Inspect MySQL:

```bash
dbprobe inspect 'mysql://dbprobe:password@127.0.0.1:3306/shop' \
  --format=json \
  --sample-window=1s
```

Request an estimated, sanitized MySQL plan:

```bash
dbprobe explain 'mysql://dbprobe:password@127.0.0.1:3306/shop' \
  --statement 'SELECT * FROM shop.orders WHERE customer_id = 1' \
  --format=json
```

`dbprobe explain` does **not** use `EXPLAIN ANALYZE`. MySQL plan inspection accepts one conservative `SELECT`, runs `EXPLAIN FORMAT=JSON` inside a bounded read-only transaction, rolls it back, and sanitizes the JSON plan before rendering. Literal conditions and unknown scalar plan fields are not emitted.

MySQL URI options are deliberately restricted to diagnostic connection settings such as `tls`, `timeout`, `readTimeout`, and `writeTimeout`. Options that expand driver behavior, including multi-statements and local-file access, are rejected.

The source-level history-aware command graph is implemented but intentionally not exposed by the driverless production composition. Once the concrete SQLite binding passes its persistence gate, repeated inspections will populate the platform data file and the default CLI will expose:

```bash
dbprobe diff 'mysql://dbprobe:password@127.0.0.1:3306/shop' --format=json
```

This staged composition prevents the CLI from presenting persistent behavior before a verified concrete SQLite connector exists.

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

## Non-relational architecture probe

`test/architecture/mongodb_probe_test.go` is a test-only MongoDB semantic adapter. It uses MongoDB-native capabilities, object kinds and signals (`mongodb.wiredtiger`, `mongodb.query_shape`, replica-set/oplog concepts) through the same collection, finding and temporal pipelines. It is architecture evidence, not production MongoDB support.

## Development

Full local gate:

```bash
make ci
```

The current pre-driver integration tree passes this gate with Go 1.25, including module tidiness, repository formatting, `go vet`, normal tests, race tests, binary build, and CLI smoke coverage.

MySQL integration gate:

```bash
make test-mysql
```

The MySQL matrix uses pinned MySQL 8.0.46 and 8.4.11 containers and is intended to verify read-only access, capability truthfulness, adapter contracts, schema fingerprint stability, sanitized plan output, versioned reports, and credential/privacy behavior.

### Remaining acceptance gates

The following gates remain intentionally open:

```text
Pin modernc.org/sqlite and register its database/sql driver at the composition root
Bind the default CLI to the owned SQLite history-store factory
SQLite live Go on-disk create / save / close / reopen / diff verification
Final driver-inclusive Go 1.25 make ci and binary smoke gate
MySQL 8.0.46 Docker integration
MySQL 8.4.11 Docker integration
```

See `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`, `docs/adr/ADR-013-sqlite-driver-selection.md`, and the implementation plans under `docs/superpowers/plans/` for the architecture contract, locked decisions and execution details.
