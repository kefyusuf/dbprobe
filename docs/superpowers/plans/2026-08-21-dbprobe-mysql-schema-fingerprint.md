# dbprobe MySQL Schema Fingerprint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a deterministic, privacy-preserving MySQL structural-schema fingerprint that can be compared temporally without leaking raw DDL/schema expressions into reports.

**Architecture:** The MySQL adapter reads bounded, deterministic metadata from `information_schema`, canonicalizes records with length-prefixed fields, and hashes the canonical stream with SHA-256. Only the versioned opaque digest crosses the adapter boundary as `mysql.schema.structural_fingerprint`; core/temporal code does not know MySQL schema concepts or the hash inputs. The v1 coverage includes objects, columns, indexes, key/constraint relationships, CHECK clauses, and referential actions. It intentionally does not claim to fingerprint routines, triggers, scheduled events, or raw view definitions.

**Tech Stack:** Go 1.25 standard library (`crypto/sha256`), existing MySQL `Queryer`, MySQL 8.0/8.4 `information_schema`.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Global Constraints

- No schema-specific logic enters `internal/core`.
- Fingerprint signal is engine-native: `mysql.schema.structural_fingerprint`.
- Hash format is versioned: `v1:sha256:<lowercase-hex>`.
- Metadata ordering must be deterministic in SQL and canonicalization.
- Canonical encoding uses length-prefixed fields; delimiter concatenation is forbidden.
- Raw canonical records, defaults, generated expressions, CHECK expressions, or referential metadata are never emitted as observations/findings.
- A collector failure produces no fabricated fingerprint.
- Capability discovery must prove every metadata surface used by the collector is readable.
- Each metadata field is bounded to 1 MiB, each canonical record to 4 MiB, each group to 100,000 rows, and the full canonical metadata set to 64 MiB.
- v1 does not include routines, triggers, scheduled events, or raw view definitions; this limitation is documented rather than hidden.

---

## File Structure

```text
adapters/mysql/collectors/schema_fingerprint.go
adapters/mysql/collectors/schema_fingerprint_test.go
adapters/mysql/collectors/schema_fingerprint_limits_test.go
adapters/mysql/capabilities.go
adapters/mysql/capabilities_test.go
adapters/mysql/runtime.go
test/integration/mysql/init.sql
test/integration/mysql/mysql_integration_test.go
docs/superpowers/plans/2026-08-21-dbprobe-mysql-schema-fingerprint.md
```

---

### Task 1: Canonical fingerprint algorithm

**Files:**
- Create: `adapters/mysql/collectors/schema_fingerprint.go`
- Create: `adapters/mysql/collectors/schema_fingerprint_test.go`
- Create: `adapters/mysql/collectors/schema_fingerprint_limits_test.go`

**Interfaces:**

```go
func NewSchemaFingerprint(query Queryer, database string) collector.Collector
```

Produces exactly one observation:

```text
key:         mysql.schema.structural_fingerprint
object:      mysql.schema:<database>
value:       v1:sha256:<hex>
exactness:   scraped
sensitivity: metadata
```

**Metadata record groups:**

1. `information_schema.tables`
   - schema, table name, table type, engine, row format, table collation
2. `information_schema.columns`
   - table, ordinal position, column name, column type, nullable, default, extra, collation, generation expression
3. `information_schema.statistics`
   - table, index name, non-unique, sequence, column/expression, prefix length, collation/direction, index type, visibility
4. `information_schema.table_constraints` + `key_column_usage`
   - table, constraint name/type, column ordinal, column, referenced table/column
5. `information_schema.check_constraints`
   - CHECK constraint schema/table/name and `CHECK_CLAUSE`
6. `information_schema.referential_constraints`
   - FK constraint identity, unique constraint identity, match option, update rule, delete rule

- [x] Write RED tests proving identical metadata with different fixture row order yields the same digest after deterministic record sorting/canonicalization.
- [x] Write RED tests proving column/index/FK/CHECK/referential-action structural changes change the digest.
- [x] Write RED tests proving delimiter-like metadata values cannot create canonical collisions.
- [x] Implement length-prefixed canonical record encoding and SHA-256 digest.
- [x] Never emit raw metadata; return only the opaque fingerprint observation.
- [x] Enforce 1 MiB field, 4 MiB record, 100,000-row/group, and 64 MiB total canonical metadata limits with fail-closed behavior.
- [x] Run focused collector normal tests and race detector locally.

---

### Task 2: MySQL metadata collector queries

**Files:**
- Continue: `adapters/mysql/collectors/schema_fingerprint.go`
- Continue tests.

- [x] Add six deterministic queries with `WHERE ... = ?` and explicit `ORDER BY` clauses.
- [x] Coalesce nullable metadata to stable empty strings and cast non-string values where needed so the existing `Rows` abstraction remains valid.
- [x] Bound scope to the selected logical database; do not scan other schemas.
- [x] Treat any query/scan/bounds error as collector failure rather than hashing partial metadata.
- [x] Test that all six query calls receive the selected database and that an error in any record group returns no fingerprint.
- [x] Run focused collector normal/race tests locally.

---

### Task 3: Capability truthfulness and runtime registration

**Files:**
- Modify: `adapters/mysql/capabilities.go`
- Modify: `adapters/mysql/capabilities_test.go`
- Modify: `adapters/mysql/runtime.go`

**Capability:** `mysql.schema_fingerprint`

- [x] Add a probe that touches every v1 metadata surface: `tables`, `columns`, `statistics`, `table_constraints`, `key_column_usage`, `check_constraints`, and `referential_constraints`.
- [x] Advertise `mysql.schema_fingerprint` only when the probe succeeds.
- [x] Keep `schema.objects` independent so restricted targets can still expose partial schema diagnostics without claiming fingerprint visibility.
- [x] Add `NewSchemaFingerprint(...)` to runtime collectors; descriptor requires only `mysql.schema_fingerprint`.
- [x] Verify capability independence and source coverage with local normal/race harness tests.

---

### Task 4: Integration acceptance

**Files:**
- Modify: `test/integration/mysql/init.sql`
- Modify: `test/integration/mysql/mysql_integration_test.go`

- [x] Add an actual CHECK constraint and explicit FK `ON UPDATE`/`ON DELETE` actions to the shared MySQL 8.0/8.4 fixture so extended metadata paths are non-empty.
- [x] Require `mysql.schema_fingerprint` in both MySQL 8.0.46 and 8.4.11 fixture targets.
- [x] Assert inspect emits exactly one `mysql.schema.structural_fingerprint` for `mysql.schema:shop`, with a lowercase `v1:sha256:<64 hex>` value.
- [x] Add a second inspect assertion requiring fingerprint stability without schema mutation.
- [x] Keep the dbprobe fixture user read-only; do not mutate schema from the diagnostic user.
- [ ] Execute the live Docker matrix when Docker/Go 1.25 environment becomes available.

## Acceptance

- Fingerprint implementation is deterministic and privacy-preserving.
- Raw schema defaults/expressions/CHECK clauses/referential definitions do not enter the report.
- Core has zero MySQL schema knowledge.
- Capability truthfully represents visibility of all required metadata surfaces.
- Algorithm/coverage is explicitly versioned and documented.
- Local focused collector and capability tests pass with the race detector.
- Full MySQL 8.0/8.4 integration remains environment-pending until Docker is available.
