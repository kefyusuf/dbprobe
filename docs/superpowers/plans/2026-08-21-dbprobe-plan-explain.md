# dbprobe Plan-Only EXPLAIN Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a database-agnostic, privacy-preserving plan-inspection contract and a MySQL implementation that returns sanitized estimated `EXPLAIN FORMAT=JSON` metadata without intentionally executing the target query.

**Architecture:** Plan inspection is an optional SDK capability implemented by adapter runtimes; `adapter.Runtime` is not widened. The application layer resolves a target, requires `query.explain`, type-asserts `adapter.PlanExplainer`, and renders only results explicitly marked `Estimated=true` and `Sanitized=true`. MySQL accepts a conservative single `SELECT`, executes only `EXPLAIN FORMAT=JSON` inside a bounded read-only transaction, rolls the transaction back, and sanitizes the JSON plan through a scalar allowlist before it crosses the adapter boundary.

**Tech Stack:** Go 1.25, existing adapter SDK/registry, Cobra CLI, MySQL 8.0/8.4, `go-sql-driver/mysql v1.10.0`.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Global Constraints

- Core/application code must not import the MySQL driver or concrete MySQL adapter.
- `adapter.Runtime` remains unchanged; plan inspection is an optional interface.
- No `EXPLAIN ANALYZE` execution path exists.
- MVP accepts one `SELECT` statement only; `WITH`, DML, `EXPLAIN`, multi-statements, locking clauses, `INTO`, and session assignment are rejected.
- Statement input is limited to 64 KiB.
- MySQL plan execution is bounded to 5 seconds.
- MySQL uses `BeginTx(... ReadOnly:true)` and always rolls back after retrieving the plan.
- Raw MySQL JSON plans are limited to 1 MiB before sanitization.
- Raw condition/expression/literal fields are not emitted. Scalar values survive only through explicit safe-key allowlists.
- `ExplainRequest.Statement` is never JSON-serialized.
- Application/surfaces reject any adapter result with `Sanitized=false`.
- MySQL connection options continue to reject `multiStatements`.
- Adapter-originated errors must not include credentials, raw target URLs, or user SQL.
- JSON output is versioned as `dbprobe.explain/v1alpha1`.

---

## File Structure

```text
sdk/adapter/explain.go
sdk/adapter/explain_test.go
internal/app/explain/service.go
internal/app/explain/service_test.go
adapters/mysql/explain.go
adapters/mysql/explain_test.go
adapters/mysql/explain_sanitize.go
adapters/mysql/explain_sanitize_test.go
internal/surfaces/json/explain.go
internal/surfaces/json/explain_test.go
internal/surfaces/terminal/explain.go
internal/surfaces/terminal/explain_test.go
cmd/dbprobe/registry.go
cmd/dbprobe/explain.go
cmd/dbprobe/explain_test.go
cmd/dbprobe/inspect.go
cmd/dbprobe/root.go
```

---

### Task 1: Optional SDK plan-inspection contract

**Files:**
- Create: `sdk/adapter/explain.go`
- Create: `sdk/adapter/explain_test.go`

**Interfaces:**

```go
type ExplainRequest struct {
    Statement string `json:"-"`
}

type ExplainResult struct {
    Engine    string `json:"engine"`
    Format    string `json:"format"`
    Estimated bool   `json:"estimated"`
    Sanitized bool   `json:"sanitized"`
    Plan      string `json:"plan"`
}

type PlanExplainer interface {
    ExplainPlan(context.Context, ExplainRequest) (ExplainResult, error)
}
```

- [x] Define the optional contract without modifying `adapter.Runtime`.
- [x] Verify the request statement is excluded from JSON serialization.
- [x] Verify safety metadata is machine-readable on results.
- [x] Run the dependency-free SDK contract tests and race detector in a local harness.

---

### Task 2: Conservative MySQL plan-only execution envelope

**Files:**
- Create: `adapters/mysql/explain.go`
- Create: `adapters/mysql/explain_test.go`

**Validation contract:**

```text
TrimSpace
must start with SELECT followed by whitespace/end
reject empty input
reject >64 KiB
reject NUL
reject semicolon/multi-statement
reject EXPLAIN / EXPLAIN ANALYZE
reject WITH for MVP
reject INSERT/UPDATE/DELETE/REPLACE/CALL/SET
reject SELECT ... INTO
reject SELECT ... FOR UPDATE
reject SELECT ... FOR SHARE
reject LOCK IN SHARE MODE
reject := session assignment
```

**Execution contract:**

```text
context timeout: 5s
BeginTx(ReadOnly=true)
EXPLAIN FORMAT=JSON <validated SELECT>
Scan JSON plan
ROLLBACK
never COMMIT
```

- [x] Write RED validation/executor tests.
- [x] Implement conservative validation.
- [x] Implement read-only transaction execution.
- [x] Verify `BeginTx(ReadOnly=true)`, query execution, and rollback with a standard-library fake SQL driver.
- [x] Verify executor errors do not echo statement text.
- [x] Run local normal and race tests for the dependency-free execution harness.

---

### Task 3: MySQL plan privacy sanitizer

**Files:**
- Create: `adapters/mysql/explain_sanitize.go`
- Create: `adapters/mysql/explain_sanitize_test.go`

**Input:** Raw `EXPLAIN FORMAT=JSON` payload.

**Output:** Sanitized JSON string with structural optimizer metadata only.

Safe string metadata includes:

```text
schema_name
table_name
access_type
possible_keys
key
key_length
used_key_parts
used_columns
using_join_buffer
query_cost
read_cost
eval_cost
prefix_cost
sort_cost
data_read_per_join
```

Safe numeric/boolean metadata is also explicit allowlist-only, including cardinality/selectivity and structural optimizer flags. Unknown scalar strings, numbers, and booleans are dropped.

- [x] Bound raw plan size to 1 MiB.
- [x] Require a single top-level JSON object.
- [x] Reject trailing JSON documents.
- [x] Drop `attached_condition`, expression strings, constant `ref` arrays, unknown scalar fields, and literal-like values.
- [x] Preserve safe table/index/cost/cardinality metadata.
- [x] Reject a plan that contains no safe metadata after sanitization.
- [x] Run sanitizer normal tests and race detector locally.

---

### Task 4: Engine-agnostic application service and renderers

**Files:**
- Create: `internal/app/explain/service.go`
- Create: `internal/app/explain/service_test.go`
- Create: `internal/surfaces/json/explain.go`
- Create: `internal/surfaces/json/explain_test.go`
- Create: `internal/surfaces/terminal/explain.go`
- Create: `internal/surfaces/terminal/explain_test.go`

**Report:**

```go
type Report struct {
    SchemaVersion string
    Target        adapter.TargetMetadata
    Format        string
    Estimated     bool
    Sanitized     bool
    Plan          string
}
```

- [x] Require `query.explain` capability.
- [x] Require the optional `adapter.PlanExplainer` interface.
- [x] Reject engine mismatch.
- [x] Reject empty format/plan.
- [x] Reject `Estimated=false`.
- [x] Reject `Sanitized=false`.
- [x] Keep the original statement out of the report.
- [x] Expose `sanitized:true` in JSON and terminal output.
- [x] Run application/renderer dependency-free normal and race tests in the local harness.

---

### Task 5: CLI `dbprobe explain`

**Files:**
- Create: `cmd/dbprobe/registry.go`
- Create: `cmd/dbprobe/explain.go`
- Create: `cmd/dbprobe/explain_test.go`
- Modify: `cmd/dbprobe/inspect.go`
- Modify: `cmd/dbprobe/root.go`

**CLI:**

```text
dbprobe explain <target> --statement "SELECT ..." --format=json|text
```

- [x] Centralize CLI adapters in `newAdapterRegistry()` so inspect/explain cannot drift.
- [x] Validate format and non-empty statement before building/opening the adapter registry.
- [x] Route CLI through the engine-agnostic application explain service; CLI never constructs MySQL SQL.
- [x] Add injected-registry command tests for JSON rendering and validation ordering.
- [x] Require `sanitized:true` in CLI JSON fixtures.
- [ ] Run full Cobra command tests when the Go/Cobra dependency environment is available.
- [ ] Add/execute binary smoke coverage when the full Go 1.25 environment is available.

---

## Acceptance

Source-level acceptance requires:

- `adapter.Runtime` unchanged and `PlanExplainer` optional.
- MySQL execution path builds only `EXPLAIN FORMAT=JSON <validated SELECT>`.
- MySQL execution uses a 5-second context and read-only transaction followed by rollback.
- No production execution path uses `EXPLAIN ANALYZE`.
- Multi-statement, non-SELECT, locking, `INTO`, and assignment forms are rejected before database access.
- Raw plan literals/conditions never cross the adapter boundary.
- Unknown plan scalar fields are dropped rather than trusted.
- Adapter results must be `Estimated=true` and `Sanitized=true` before application rendering.
- `dbprobe.explain/v1alpha1` remains versioned and does not contain the input statement.
- Existing inspect functionality continues to use the same shared adapter registry.

Environment acceptance remains pending until available:

```text
Go 1.25 full suite
Cobra CLI compile/tests
MySQL 8.0 live EXPLAIN integration
MySQL 8.4 live EXPLAIN integration
credential/privacy smoke tests
```

No merge-ready claim is made before those environment gates run successfully.
