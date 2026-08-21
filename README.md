# dbprobe

Database intelligence runtime for deterministic, read-only diagnostics across multiple database engines.

> MySQL adapter source implementation is feature-complete for the MVP scope. Full Go 1.25 and Docker MySQL 8.0/8.4 acceptance remain pending because the current execution environment has no Actions quota, outbound DNS, or Docker runtime.

## Architecture

`dbprobe` is database-agnostic at its core. Database-specific connectivity, SQL, telemetry and deterministic rules live behind adapter contracts.

```text
CLI / JSON / MCP / AI
        |
Application Runtime
        |
Core: capabilities / signals / findings / deltas
        |
Adapter SDK
   |       |        |
 MySQL   MongoDB  PostgreSQL ...
```

The adapter SDK is intentionally pre-1.0 until a non-relational adapter validates the abstraction boundary.

## Current implementation

- Foundation runtime: fake adapter, capability-aware collection planner, reset-safe counter deltas, versioned JSON report and CLI.
- First production adapter: MySQL.
- MySQL primary target: 8.4 LTS.
- MySQL 8.0.46 is retained as legacy compatibility coverage; MySQL 8.0 reached EOL in April 2026.
- Optional plan-only explain contract with a MySQL `EXPLAIN FORMAT=JSON` implementation is under source-complete/environment-pending validation.
- MongoDB is planned as the first non-relational architecture-validation adapter.

## CLI

```bash
dbprobe inspect fake://local

dbprobe inspect 'mysql://dbprobe:password@127.0.0.1:3306/shop' \
  --format=json \
  --sample-window=1s

# Estimated, sanitized optimizer plan; does not use EXPLAIN ANALYZE.
dbprobe explain 'mysql://dbprobe:password@127.0.0.1:3306/shop' \
  --statement "SELECT * FROM orders WHERE customer_id = 42" \
  --format=json
```

MySQL URI options are deliberately restricted to diagnostic connection settings: `tls`, `timeout`, `readTimeout`, and `writeTimeout`. Options that expand driver behavior such as multi-statements or local-file access are rejected.

### Plan-only explain safety

`dbprobe explain` is intentionally narrower than a general SQL client:

- accepts a single `SELECT` statement only;
- rejects `EXPLAIN`, `EXPLAIN ANALYZE`, DML, `WITH` in MVP, multi-statements, locking clauses, `INTO`, and session assignment;
- limits input to 64 KiB and plan execution to 5 seconds;
- runs MySQL plan inspection inside a read-only transaction and rolls it back;
- executes only `EXPLAIN FORMAT=JSON <validated SELECT>`;
- limits raw plan JSON to 1 MiB;
- strips condition/expression/literal fields and unknown scalar values through a privacy allowlist;
- reports `estimated: true` and `sanitized: true` explicitly;
- never serializes the input statement as part of the explain report.

The sanitized plan intentionally prioritizes safe structural optimizer metadata such as table/index/access type, cardinality, selectivity, and cost information over reproducing MySQL's raw JSON payload.

## Safety model

- Deterministic collection and findings; AI does not decide diagnoses.
- Inspection paths are read-only and do not run remediation.
- Query evidence uses normalized MySQL digest text rather than literal SQL.
- Transaction query text and replication error-message text are not collected.
- Plan output is privacy-sanitized before leaving the MySQL adapter.
- Credential-bearing targets are redacted from errors.
- Missing privileges reduce capabilities instead of pretending the inspection is complete.

## Development

```bash
make ci
```

MySQL integration is opt-in:

```bash
make test-mysql
```

The integration matrix uses pinned MySQL 8.0.46 and 8.4.11 containers and verifies read-only access, adapter contracts, shared JSON output, credential redaction, and sanitized plan-only explain behavior.

See `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md` and the implementation plans under `docs/superpowers/plans/` for the architecture contract and execution details.
