# dbprobe Bounded Concurrent Collection Planner Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans task-by-task. Steps use checkbox syntax.

**Goal:** Make independent collectors execute concurrently with a strict concurrency bound while preserving capability filtering, partial failure, deterministic output, central counter sampling, and reset safety.

**Architecture:** `internal/core/collection.Planner` owns concurrency. Adapters remain unaware of goroutine scheduling and continue exposing ordinary `Collector` instances. Snapshot collectors execute in one bounded batch; eligible counter collectors execute sample A in a bounded batch, the planner waits once for the sample window, then executes sample B in a second bounded batch using the same collector instances.

**Tech Stack:** Go standard library only (`context`, `sync`, channels). No external dependency.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Constraints

- Preserve `collection.New(waiter, now)`.
- Default maximum concurrency is `4`.
- Add `(*Planner).WithMaxConcurrency(int) *Planner`; values `<1` normalize to `1`.
- No database/adapter imports in core.
- One collector failure must not cancel unrelated collectors.
- Context cancellation may stop queued work and must return the context error.
- Counter sample A and sample B for the same collector must never overlap.
- Wait exactly once between counter phases when at least one eligible counter collector exists.
- Output observations, deltas, and warnings remain deterministically sorted.
- Counter decreases remain reset warnings and never emit negative deltas.

### Task 1: Concurrency contract tests

**Files:**
- Modify: `internal/core/collection/planner_test.go`

- [ ] Add a blocking collector fixture that tracks current and maximum simultaneous calls.
- [ ] Add `TestPlannerRunsSnapshotCollectorsWithConcurrencyBound`: 6 snapshot collectors, `WithMaxConcurrency(2)`, assert max in-flight is exactly 2 and all evidence is retained.
- [ ] Add `TestPlannerRunsCounterPhasesWithConcurrencyBoundAndSingleWait`: at least 4 counters, bound 2, assert sample A batch completes before the single wait and no sample B starts before wait release.
- [ ] Add `TestPlannerCancellationStopsQueuedCollectors`: bound 1, block first collector, cancel context, assert planner returns `context.Canceled` and queued collectors do not all execute.
- [ ] Run package tests and verify RED against the sequential implementation.

### Task 2: Bounded batch executor

**Files:**
- Modify: `internal/core/collection/planner.go`

- [ ] Add `maxConcurrency int` to `Planner`, default `4` in `New`.
- [ ] Add `WithMaxConcurrency` preserving fluent use.
- [ ] Implement an internal bounded `runBatch` helper using worker goroutines or a semaphore. Results carry the original collector descriptor/index rather than mutating shared result slices.
- [ ] Snapshot batch: execute only capability-eligible snapshot collectors.
- [ ] Counter A batch: execute eligible counter collectors and retain successful sample-A observations keyed by collector ID + signal identity.
- [ ] Wait once if at least one counter collector is eligible; context cancellation aborts before phase B.
- [ ] Counter B batch: run the same eligible collector list, append successful current observations, compute deltas from successful A/B pairs.
- [ ] Convert collector errors to warnings without cancelling sibling work.
- [ ] Sort final outputs exactly as before.
- [ ] Run unit and race tests.

### Task 3: Compatibility and acceptance

- [ ] Existing planner tests still pass unchanged except where concurrency-aware fixtures are added.
- [ ] `go test -race` reports no data race in planner package.
- [ ] Fake adapter vertical slice remains compatible because public constructor signatures are unchanged.
- [ ] Architecture guard remains green.

## Definition of Done

Planner concurrency is engine-agnostic, bounded, cancellation-aware, partial-failure-safe, centralizes the sample window, and does not change output contracts or require adapter changes.
