# dbprobe Plan-Only EXPLAIN Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a database-agnostic plan-inspection contract and a MySQL implementation that returns estimated `EXPLAIN FORMAT=JSON` output without executing the target query.

**Architecture:** Plan inspection is an optional SDK capability implemented by adapter runtimes; `adapter.Runtime` itself is not widened. The application layer resolves a target, requires `query.explain`, type-asserts the optional `adapter.PlanExplainer`, and returns a versioned generic report. MySQL accepts a conservative single `SELECT` statement only and always prepends `EXPLAIN FORMAT=JSON`; `EXPLAIN ANALYZE` is never generated or accepted.

**Tech Stack:** Go 1.25, existing adapter SDK/registry, Cobra CLI, MySQL 8.0/8.4, `go-sql-driver/mysql v1.10.0`.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Global Constraints

- Core/application code must not import the MySQL driver or concrete MySQL adapter.
- Plan inspection is read-only and estimated; no `EXPLAIN ANALYZE`, query execution, DDL, DML execution, or remediation.
- MVP input is one `SELECT` statement only; CTE/DML explain support is deferred until a real SQL parser or equivalent safe contract exists.
- User SQL is never logged/persisted by the plan service.
- MySQL connection options continue to reject `multiStatements`.
- Adapter-originated errors must not include credentials or raw target URLs.
- JSON output is versioned.

---

## File Structure

```text
sdk/adapter/explain.go
internal/app/explain/service.go
internal/app/explain/service_test.go
adapters/mysql/explain.go
adapters/mysql/explain_test.go
internal/surfaces/json/explain.go
internal/surfaces/json/explain_test.go
internal/surfaces/terminal/explain.go
internal/surfaces/terminal/explain_test.go
cmd/dbprobe/explain.go
cmd/dbprobe/explain_test.go
```

---

### Task 1: Optional SDK plan-inspection contract

**Files:**
- Create: `sdk/adapter/explain.go`
- Test through application/MySQL tasks; no standalone interface-only test required.

**Interfaces:**
- Produces: `adapter.PlanExplainer`, `adapter.ExplainRequest`, `adapter.ExplainResult`.

- [ ] **Step 1: Define the optional contract**

```go
type ExplainRequest struct {
    Statement string
}

type ExplainResult struct {
    Engine    string
    Format    string
    Estimated bool
    Plan      []byte
}

type PlanExplainer interface {
    ExplainPlan(context.Context, ExplainRequest) (ExplainResult, error)
}
```

- [ ] **Step 2: Verify existing fake adapter/runtime still compiles without implementing the optional interface**

Run: `go test ./adapters/fake ./sdk/adapter`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add -- sdk/adapter/explain.go
git commit -m "feat: define optional plan explain contract"
```

---

### Task 2: MySQL conservative plan-only EXPLAIN

**Files:**
- Create: `adapters/mysql/explain.go`
- Create: `adapters/mysql/explain_test.go`

**Interfaces:**
- Consumes: `adapter.PlanExplainer` contract and the runtime-owned `*sql.DB`.
- Produces: `(*runtime).ExplainPlan(ctx, request)`.

- [ ] **Step 1: Write failing tests** asserting:
  - ordinary `SELECT` becomes exactly `EXPLAIN FORMAT=JSON <statement>`;
  - result is `Engine=mysql`, `Format=mysql-json`, `Estimated=true`;
  - leading/trailing whitespace is accepted;
  - empty input is rejected;
  - `EXPLAIN`, `EXPLAIN ANALYZE`, `WITH`, `UPDATE`, `DELETE`, `INSERT`, `REPLACE`, `CALL`, `SET`, and multi-statement inputs are rejected before database access;
  - errors do not echo the full rejected SQL.

Example:

```go
func TestValidateExplainStatementAllowsSingleSelect(t *testing.T) {
    got, err := validateExplainStatement("  SELECT * FROM orders WHERE id = 1  ")
    if err != nil { t.Fatal(err) }
    if got != "SELECT * FROM orders WHERE id = 1" { t.Fatalf("got %q", got) }
}
```

- [ ] **Step 2: Run RED**

Run: `go test ./adapters/mysql -run Explain -v`
Expected: FAIL because validation/plan methods do not exist.

- [ ] **Step 3: Implement minimal validation and plan query**

Rules:

```text
TrimSpace
must begin with SELECT followed by whitespace/end
reject any semicolon
reject NUL
prepend only: EXPLAIN FORMAT=JSON 
```

The runtime executes the generated EXPLAIN with `QueryRowContext(...).Scan(&planJSON)`. It never executes the original statement separately.

- [ ] **Step 4: Run GREEN + race**

Run:

```bash
go test ./adapters/mysql -run Explain -v
go test -race ./adapters/mysql -run Explain -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -- adapters/mysql/explain.go adapters/mysql/explain_test.go
git commit -m "feat: add safe MySQL plan-only explain"
```

---

### Task 3: Engine-agnostic explain application service and renderers

**Files:**
- Create: `internal/app/explain/service.go`
- Create: `internal/app/explain/service_test.go`
- Create: `internal/surfaces/json/explain.go`
- Create: `internal/surfaces/json/explain_test.go`
- Create: `internal/surfaces/terminal/explain.go`
- Create: `internal/surfaces/terminal/explain_test.go`

**Interfaces:**
- Produces versioned `dbprobe.explain/v1alpha1` report.

- [ ] **Step 1: Write failing service tests** using a test adapter/runtime implementing `adapter.PlanExplainer`.

Expected report fields:

```go
type Report struct {
    SchemaVersion string
    Target        adapter.TargetMetadata
    Format        string
    Estimated     bool
    Plan          []byte
}
```

Assert missing `query.explain` capability and runtime-without-`PlanExplainer` both fail explicitly.

- [ ] **Step 2: Run RED**

Run: `go test ./internal/app/explain ./internal/surfaces/json ./internal/surfaces/terminal`
Expected: FAIL because packages do not exist.

- [ ] **Step 3: Implement service and renderers**

JSON emits the versioned report. Terminal prints target/format/estimated status then the plan payload without modifying it.

- [ ] **Step 4: Run GREEN + race**

```bash
go test ./internal/app/explain ./internal/surfaces/json ./internal/surfaces/terminal
go test -race ./internal/app/explain ./internal/surfaces/json ./internal/surfaces/terminal
```

- [ ] **Step 5: Commit**

```bash
git add -- internal/app/explain internal/surfaces/json/explain* internal/surfaces/terminal/explain*
git commit -m "feat: add plan explain application surface"
```

---

### Task 4: CLI `dbprobe explain`

**Files:**
- Create: `cmd/dbprobe/explain.go`
- Create: `cmd/dbprobe/explain_test.go`
- Modify: `cmd/dbprobe/root.go`

**CLI:**

```text
dbprobe explain <target> --statement "SELECT ..." --format=json|text
```

- [ ] **Step 1: Write failing CLI tests** for required target/statement, format validation, and successful fake plan-explainer composition test.
- [ ] **Step 2: Run RED** with `go test ./cmd/dbprobe -run Explain -v`.
- [ ] **Step 3: Add command** using the same adapter registry composition root as inspect. The command calls the application explain service; it never constructs MySQL SQL itself.
- [ ] **Step 4: Run CLI tests and existing fake inspect regression tests**.
- [ ] **Step 5: Add smoke command to Makefile only after full Go 1.25/dependencies are available; do not claim that gate under the current environment.**
- [ ] **Step 6: Commit** with `feat: add dbprobe explain command`.

## Acceptance

- `adapter.Runtime` remains unchanged; plan explain is optional.
- MySQL plan inspection executes only `EXPLAIN FORMAT=JSON SELECT ...`.
- No `EXPLAIN ANALYZE` string exists in production execution paths.
- Multi-statement and non-SELECT input is rejected before database access.
- `dbprobe.explain/v1alpha1` JSON is stable/versioned.
- Existing inspect functionality remains unchanged.
- Full Go 1.25 and live MySQL acceptance stay pending until environment quota/Docker access returns.
