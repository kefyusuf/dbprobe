# dbprobe Generic Core Findings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move genuinely portable deterministic findings out of engine adapters into a core rule pack, starting with `core.connection_saturation`.

**Architecture:** Core rules consume portable `core.*` observations and must not branch on engine names or import adapters/drivers. The inspect application evaluates core rules first, then adapter-native rules. Applicability is evidence-driven: `core.connection_saturation` requires the portable used/limit signals rather than a MySQL-specific capability.

**Tech Stack:** Go 1.25, existing SDK `finding`/`signal` contracts, inspect application service.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Global Constraints

- `internal/core/findings` must not import adapters, `database/sql`, or engine drivers.
- Generic rule IDs use `core.*` only when semantics are portable.
- Missing or invalid evidence produces no finding; rules do not synthesize defaults.
- Connection saturation thresholds remain warning >=85% and critical >=95%.
- Generic rules run before adapter rules.
- The MySQL-specific duplicate rule is removed on the MySQL branch before integration/merge to prevent duplicate findings.

---

### Task 1: Generic `core.connection_saturation` rule

**Files:**
- Create: `internal/core/findings/connections.go`
- Create: `internal/core/findings/connections_test.go`
- Create: `internal/core/findings/rules.go`

**Interfaces:**
- Produces: `corefindings.Rules() []finding.Rule`.

- [ ] Write RED tests for 84%, 85%, 95%, missing limit, zero limit, and object identity.
- [ ] Implement a rule that reads only `core.connections.used` and `core.connections.limit` for the same object.
- [ ] `Requires()` returns no engine-specific capabilities.
- [ ] Run normal and race tests.

---

### Task 2: Inspect application generic-rule wiring

**Files:**
- Modify: `internal/app/inspect/service.go`
- Create: `internal/app/inspect/generic_findings_test.go`

- [ ] Write a test adapter/runtime emitting `core.connections.used=90` and `core.connections.limit=100` with no adapter rules.
- [ ] Verify RED because inspect currently evaluates only `runtime.Rules()`.
- [ ] Prepend `corefindings.Rules()` to adapter rules in the inspect service.
- [ ] Verify the report contains exactly one `core.connection_saturation` warning.
- [ ] Verify existing fake-adapter inspect behavior remains unchanged.
- [ ] Run normal and race tests.

---

### Task 3: Cross-branch MySQL compatibility gate

**Files on `feat/mysql-adapter-mvp`:**
- Remove the MySQL-local `connectionSaturationRule` from `adapters/mysql/findings/rules.go`/tests after the generic core branch is integrated into its base.

- [ ] Do not perform this removal on the isolated generic-core branch because it does not contain the MySQL adapter.
- [ ] Record the required follow-up in the MySQL plan/PR until branches are integrated.

## Acceptance

- `core.connection_saturation` implementation has zero MySQL knowledge.
- A non-MySQL adapter that emits the same portable signals receives the same deterministic finding without copying the rule.
- Adapter-native findings still run through the same inspect report.
- No duplicate generic finding exists after MySQL branch integration.
