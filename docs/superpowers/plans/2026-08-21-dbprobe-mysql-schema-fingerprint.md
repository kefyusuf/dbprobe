# dbprobe MySQL Schema Fingerprint Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a deterministic, privacy-preserving MySQL structural-schema fingerprint that can be compared temporally without leaking raw DDL/schema expressions into reports.

**Architecture:** The MySQL adapter reads bounded, deterministic metadata from `information_schema`, canonicalizes records with length-prefixed fields, and hashes the canonical stream with SHA-256. Only the versioned opaque digest crosses the adapter boundary as `mysql.schema.structural_fingerprint`; core/temporal code does not know MySQL schema concepts or the hash inputs. The v1 coverage is explicitly limited to tables/views-as-objects, columns, indexes, and key/constraint relationships; it does not claim to fingerprint routines, triggers, events, or raw view definitions.

**Tech Stack:** Go 1.25 standard library (`crypto/sha256`), existing MySQL `Queryer`, MySQL 8.0/8.4 `information_schema`.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Global Constraints

- No schema-specific logic enters `internal/core`.
- Fingerprint signal is engine-native: `mysql.schema.structural_fingerprint`.
- Hash format is versioned: `v1:sha256:<lowercase-hex>`.
- Metadata ordering must be deterministic in SQL and canonicalization.
- Canonical encoding uses length-prefixed fields; delimiter concatenation is forbidden.
- Raw canonical records, defaults, expressions, or constraint metadata are never emitted as observations/findings.
- A collector failure produces no fabricated fingerprint.
- Capability discovery must prove every metadata surface used by the collector is readable.
- v1 does not include routines, triggers, scheduled events, or raw view definitions; this limitation is documented rather than hidden.

---

## File Structure

```text
adapters/mysql/collectors/schema_fingerprint.go
adapters/mysql/collectors/schema_fingerprint_test.go
adapters/mysql/capabilities.go
adapters/mysql/capabilities_test.go
adapters/mysql/runtime.go
test/integration/mysql/mysql_integration_test.go
docs/superpowers/plans/2026-08-21-dbprobe-mysql-schema-fingerprint.md
```

---

### Task 1: Canonical fingerprint algorithm

**Files:**
- Create: `adapters/mysql/collectors/schema_fingerprint.go`
- Create: `adapters/mysql/collectors/schema_fingerprint_test.go`

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

- [ ] Write RED tests proving identical metadata with different fixture row order yields the same digest after deterministic record sorting/canonicalization.
- [ ] Write RED tests proving structural changes (type, index direction/prefix, FK target, default/generation metadata) change the digest.
- [ ] Write RED tests proving delimiter-like metadata values cannot create canonical collisions.
- [ ] Implement length-prefixed canonical record encoding and SHA-256 digest.
- [ ] Never emit raw metadata; return only the opaque fingerprint observation.
- [ ] Run collector normal tests and race detector.

---

### Task 2: MySQL metadata collector queries

**Files:**
- Continue: `adapters/mysql/collectors/schema_fingerprint.go`
- Continue tests.

- [ ] Add four deterministic queries with `WHERE ... = ?` and explicit `ORDER BY` clauses.
- [ ] Coalesce nullable metadata to stable empty strings and cast non-string values where needed so the existing `Rows` abstraction remains valid.
- [ ] Bound scope to the selected logical database; do not scan other schemas.
- [ ] Treat any query/scan error as collector failure rather than hashing partial metadata.
- [ ] Test that all query calls receive the selected database and that an error in any record group returns no fingerprint.
- [ ] Run collector normal/race tests.

---

### Task 3: Capability truthfulness and runtime registration

**Files:**
- Modify: `adapters/mysql/capabilities.go`
- Modify: `adapters/mysql/capabilities_test.go`
- Modify: `adapters/mysql/runtime.go`

**Capability:** `mysql.schema_fingerprint`

- [ ] Add a probe that touches the exact metadata surfaces required by the v1 collector (`tables`, `columns`, `statistics`, `table_constraints`, `key_column_usage`).
- [ ] Advertise `mysql.schema_fingerprint` only when the probe succeeds.
- [ ] Add `NewSchemaFingerprint(...)` to runtime collectors; descriptor requires only `mysql.schema_fingerprint`.
- [ ] Verify restricted targets simply skip the collector through the planner.
- [ ] Run capability/collector focused tests.

---

### Task 4: Integration acceptance

**Files:**
- Modify: `test/integration/mysql/mysql_integration_test.go`

- [ ] Require `mysql.schema_fingerprint` in both MySQL 8.0.46 and 8.4.11 fixture targets.
- [ ] Assert inspect emits exactly one `mysql.schema.structural_fingerprint` observation for `mysql.schema:shop`.
- [ ] Assert value matches `^v1:sha256:[0-9a-f]{64}$`.
- [ ] Open the same target twice without schema mutation and assert fingerprint stability.
- [ ] Do not add a write-capable mutation test to the dbprobe user; fixture initialization is sufficient for source acceptance.
- [ ] Execute only when Docker/Go 1.25 environment becomes available; until then mark this gate pending.

## Acceptance

- Fingerprint implementation is deterministic and privacy-preserving.
- Raw schema defaults/expressions/definitions do not enter the report.
- Core has zero MySQL schema knowledge.
- Capability truthfully represents visibility of all required metadata surfaces.
- Algorithm/coverage is explicitly versioned and documented.
- Full MySQL 8.0/8.4 integration remains environment-pending until Docker is available.
