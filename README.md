# dbprobe

Database intelligence runtime for deterministic, read-only diagnostics, temporal analysis, CI/agent surfaces, and database-specific intelligence behind modular adapters.

> **v0.1 integration status:** source-level integration is in progress on `integration-v0.1`. Foundation, bounded collection concurrency, generic core findings, temporal intelligence, the MySQL MVP adapter, MySQL schema fingerprinting, safe plan-only EXPLAIN, and a test-only MongoDB semantic probe are integrated. Full Go 1.25 and Docker MySQL 8.0/8.4 acceptance are still pending because the current execution environment has no usable GitHub Actions quota, outbound dependency access, or Docker runtime.

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
- in-memory baseline store behind the generic temporal store port;
- query-text persistence filtering.

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

## Non-relational architecture probe

`test/architecture/mongodb_probe_test.go` is a test-only MongoDB semantic adapter. It uses MongoDB-native capabilities, object kinds and signals (`mongodb.wiredtiger`, `mongodb.query_shape`, replica-set/oplog concepts) through the same collection, finding and temporal pipelines. It is architecture evidence, not production MongoDB support.

## Development

Full local gate:

```bash
make ci
```

MySQL integration gate:

```bash
make test-mysql
```

The MySQL matrix uses pinned MySQL 8.0.46 and 8.4.11 containers and is intended to verify read-only access, capability truthfulness, adapter contracts, schema fingerprint stability, sanitized plan output, versioned reports, and credential/privacy behavior.

### Environment-pending acceptance

The following gates are intentionally not claimed as complete until the required environment is available:

```text
Go 1.25: go mod tidy / gofmt / go vet / go test ./... / go test -race ./...
Cobra CLI compile and smoke tests
MySQL 8.0.46 Docker integration
MySQL 8.4.11 Docker integration
SQLite persistent baseline store
```

See `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md` and the implementation plans under `docs/superpowers/plans/` for the architecture contract, locked decisions and execution details.
