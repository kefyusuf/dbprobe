# dbprobe Foundation Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first working vertical slice of dbprobe: a database-agnostic runtime that resolves an adapter, discovers capabilities, executes snapshot/counter collectors, computes reset-aware deltas, and renders a versioned report through CLI/JSON using a deterministic fake adapter.

**Architecture:** Core operates only on SDK contracts and never imports database drivers. The fake adapter proves adapter resolution, capability filtering, centralized sampling, partial-failure behavior, security metadata, and stable machine output before MySQL is allowed into the repository. The resulting vertical slice must execute as `dbprobe inspect fake://local`.

**Tech Stack:** Go 1.25 baseline, Cobra CLI added only when the CLI task begins, Go standard-library tests, GitHub Actions. No real database driver, SQLite, MCP, LLM, runtime plugin, or daemon mode in this plan.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Global Constraints

- `internal/core` must not import MySQL, PostgreSQL, MongoDB, Cassandra, their drivers, or concrete adapters.
- `sdk` must not import concrete adapters or `internal/*` packages.
- One adapter must not import another adapter.
- `internal/surfaces` must consume the application report only; it must not import concrete adapters.
- The concrete adapter registry is wired only from the composition root under `cmd/dbprobe`.
- Findings are deterministic. AI is outside this plan and cannot influence collection or diagnosis.
- Counter timing and delta/rate calculation belong to core, not to individual adapters.
- Missing capabilities skip incompatible collectors. Collector failure degrades the report and must not erase successful evidence.
- Counter reset must never produce a negative rate.
- JSON schema version for this slice is exactly `dbprobe.inspect/v1alpha1`.
- SDK is pre-1.0 and remains changeable until the later MongoDB abstraction-validation gate.
- Every task follows red → green → full-suite → commit.
- Stage only files named by the current task.

---

## File Structure Locked By This Plan

```text
dbprobe/
├── .github/workflows/ci.yml
├── cmd/dbprobe/
│   ├── main.go
│   ├── root.go
│   └── inspect.go
├── adapters/fake/
│   ├── adapter.go
│   ├── runtime.go
│   ├── collectors.go
│   └── adapter_test.go
├── internal/
│   ├── app/inspect/
│   │   ├── service.go
│   │   └── service_test.go
│   ├── core/collection/
│   │   ├── planner.go
│   │   └── planner_test.go
│   ├── platform/adapterregistry/
│   │   ├── registry.go
│   │   └── registry_test.go
│   └── surfaces/
│       ├── json/
│       │   ├── render.go
│       │   └── render_test.go
│       └── terminal/
│           ├── render.go
│           └── render_test.go
├── sdk/
│   ├── adapter/
│   │   ├── adapter.go
│   │   ├── target.go
│   │   └── security.go
│   ├── capability/
│   │   ├── capability.go
│   │   └── capability_test.go
│   ├── collector/collector.go
│   ├── finding/finding.go
│   ├── object/ref.go
│   └── signal/
│       ├── observation.go
│       ├── delta.go
│       └── observation_test.go
├── test/
│   ├── architecture/dependencies_test.go
│   └── contract/adapter_contract_test.go
├── go.mod
├── go.sum
└── Makefile
```

Responsibility rules:

- `sdk/*`: contracts adapters may implement or consume.
- `internal/core/*`: engine-agnostic runtime logic.
- `internal/platform/*`: infrastructure wiring with no engine logic.
- `internal/app/*`: use-cases and stable application reports.
- `internal/surfaces/*`: presentation only.
- `adapters/*`: engine-specific behavior.
- `cmd/dbprobe/*`: composition root and CLI parsing.

---

### Task 1: Go foundation, CI, and architecture import guard

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `.github/workflows/ci.yml`
- Create: `test/architecture/dependencies_test.go`

**Interfaces:**
- Consumes: none.
- Produces: Go module `github.com/kefyusuf/dbprobe`, `make ci`, and a dependency-boundary test that later tasks automatically inherit.

- [ ] **Step 1: Create the dependency-free module**

Create `go.mod`:

```go
module github.com/kefyusuf/dbprobe

go 1.25
```

Run:

```bash
go test ./...
```

Expected: exit `0`; no `go.sum` exists yet because this task intentionally has no external dependency.

- [ ] **Step 2: Write the architecture guard**

Create `test/architecture/dependencies_test.go`:

```go
package architecture_test

import (
    "go/parser"
    "go/token"
    "io/fs"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "testing"
)

type rule struct {
    root      string
    forbidden []string
}

func TestDependencyBoundaries(t *testing.T) {
    rules := []rule{
        {"../../internal/core", []string{"/adapters/", "go-sql-driver/mysql", "jackc/pgx", "mongo-driver", "gocql"}},
        {"../../sdk", []string{"/adapters/", "/internal/"}},
        {"../../internal/surfaces", []string{"/adapters/"}},
    }

    for _, r := range rules {
        if _, err := os.Stat(r.root); os.IsNotExist(err) {
            continue
        }
        err := filepath.WalkDir(r.root, func(path string, d fs.DirEntry, err error) error {
            if err != nil {
                return err
            }
            if d.IsDir() || !strings.HasSuffix(path, ".go") {
                return nil
            }
            f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
            if err != nil {
                return err
            }
            for _, imp := range f.Imports {
                importPath, err := strconv.Unquote(imp.Path.Value)
                if err != nil {
                    return err
                }
                for _, forbidden := range r.forbidden {
                    if strings.Contains(importPath, forbidden) {
                        t.Errorf("%s imports forbidden dependency %q", path, importPath)
                    }
                }
            }
            return nil
        })
        if err != nil {
            t.Fatal(err)
        }
    }
}
```

Run:

```bash
go test ./test/architecture -v
```

Expected: PASS.

- [ ] **Step 3: Add repeatable local verification commands**

Create `Makefile`:

```make
.PHONY: fmt fmt-check vet test race ci

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))"

vet:
	go vet ./...

test:
	go test ./...

race:
	go test -race ./...

ci: fmt-check vet test
```

Create `.github/workflows/ci.yml`:

```yaml
name: ci

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'
          cache: true
      - run: make ci
```

Run:

```bash
make ci
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add go.mod Makefile .github/workflows/ci.yml test/architecture/dependencies_test.go
git commit -m "chore: establish Go project and architecture guard"
```

---

### Task 2: Capability, object, observation, and delta primitives

**Files:**
- Create: `sdk/capability/capability.go`
- Create: `sdk/capability/capability_test.go`
- Create: `sdk/object/ref.go`
- Create: `sdk/signal/observation.go`
- Create: `sdk/signal/delta.go`
- Create: `sdk/signal/observation_test.go`

**Interfaces:**
- Produces: `capability.Capability`, `capability.Set`, `object.Ref`, `signal.Observation`, `signal.Delta`.

- [ ] **Step 1: Write the capability-set test**

Create `sdk/capability/capability_test.go`:

```go
package capability_test

import (
    "reflect"
    "testing"

    "github.com/kefyusuf/dbprobe/sdk/capability"
)

func TestSetHasAndListsUniqueValuesInStableOrder(t *testing.T) {
    set := capability.New("storage.cache", "activity.sessions", "storage.cache")
    if !set.Has("activity.sessions") {
        t.Fatal("expected activity.sessions")
    }
    want := []capability.Capability{"activity.sessions", "storage.cache"}
    if got := set.List(); !reflect.DeepEqual(got, want) {
        t.Fatalf("List() = %#v, want %#v", got, want)
    }
}
```

Run:

```bash
go test ./sdk/capability -v
```

Expected: FAIL because package implementation does not exist.

- [ ] **Step 2: Implement capability set**

Create `sdk/capability/capability.go`:

```go
package capability

import "sort"

type Capability string

type Set struct{ values map[Capability]struct{} }

func New(values ...Capability) Set {
    set := Set{values: make(map[Capability]struct{}, len(values))}
    for _, value := range values {
        set.values[value] = struct{}{}
    }
    return set
}

func (s Set) Has(value Capability) bool {
    _, ok := s.values[value]
    return ok
}

func (s Set) HasAll(values []Capability) bool {
    for _, value := range values {
        if !s.Has(value) {
            return false
        }
    }
    return true
}

func (s Set) List() []Capability {
    out := make([]Capability, 0, len(s.values))
    for value := range s.values {
        out = append(out, value)
    }
    sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
    return out
}
```

Run `go test ./sdk/capability -v`. Expected: PASS.

- [ ] **Step 3: Write the observation test**

Create `sdk/signal/observation_test.go`:

```go
package signal_test

import (
    "testing"
    "time"

    "github.com/kefyusuf/dbprobe/sdk/object"
    "github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestNumberObservationExposesNumericValue(t *testing.T) {
    at := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
    obs := signal.NumberObservation(
        "core.connections.used",
        object.Ref{Kind: "fake.instance", ID: "local"},
        12,
        signal.UnitCount,
        signal.ExactnessScraped,
        signal.SensitivityMetadata,
        at,
    )
    got, ok := obs.Numeric()
    if !ok || got != 12 {
        t.Fatalf("Numeric() = %v, %v; want 12, true", got, ok)
    }
}
```

Run `go test ./sdk/signal -v`. Expected: FAIL.

- [ ] **Step 4: Implement object and signal contracts**

Create `sdk/object/ref.go`:

```go
package object

type Ref struct {
    Kind string `json:"kind"`
    ID   string `json:"id"`
}
```

Create `sdk/signal/observation.go`:

```go
package signal

import (
    "time"
    "github.com/kefyusuf/dbprobe/sdk/object"
)

type Key string
type Unit string
type Exactness string
type Sensitivity string

const (
    UnitCount Unit = "count"
    UnitMilliseconds Unit = "milliseconds"
    UnitBytes Unit = "bytes"
    UnitRatio Unit = "ratio"
    UnitSeconds Unit = "seconds"

    ExactnessScraped Exactness = "scraped"
    ExactnessCumulative Exactness = "cumulative"
    ExactnessSampled Exactness = "sampled"
    ExactnessEstimated Exactness = "estimated"
    ExactnessUnavailable Exactness = "unavailable"
    ExactnessReset Exactness = "reset"

    SensitivityMetadata Sensitivity = "metadata"
    SensitivityQueryShape Sensitivity = "query_shape"
    SensitivityQueryText Sensitivity = "query_text"
)

type Observation struct {
    Key Key `json:"key"`
    Object object.Ref `json:"object"`
    Unit Unit `json:"unit"`
    Exactness Exactness `json:"exactness"`
    Number *float64 `json:"number,omitempty"`
    Text *string `json:"text,omitempty"`
    Boolean *bool `json:"boolean,omitempty"`
    CollectedAt time.Time `json:"collected_at"`
    Sensitivity Sensitivity `json:"sensitivity"`
    Source string `json:"source,omitempty"`
    Reason string `json:"reason,omitempty"`
}

func NumberObservation(key Key, ref object.Ref, value float64, unit Unit, exactness Exactness, sensitivity Sensitivity, at time.Time) Observation {
    return Observation{Key: key, Object: ref, Unit: unit, Exactness: exactness, Number: &value, CollectedAt: at, Sensitivity: sensitivity}
}

func (o Observation) Numeric() (float64, bool) {
    if o.Number == nil {
        return 0, false
    }
    return *o.Number, true
}
```

Create `sdk/signal/delta.go`:

```go
package signal

import "github.com/kefyusuf/dbprobe/sdk/object"

type Delta struct {
    Key Key `json:"key"`
    Object object.Ref `json:"object"`
    Unit Unit `json:"unit"`
    Delta float64 `json:"delta"`
    RatePerSecond float64 `json:"rate_per_second"`
    WindowSeconds float64 `json:"window_seconds"`
    Exactness Exactness `json:"exactness"`
}
```

Run:

```bash
go test ./sdk/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add sdk/capability sdk/object sdk/signal
git commit -m "feat: define capability and signal primitives"
```

---

### Task 3: Collector, finding, adapter/runtime contracts and registry

**Files:**
- Create: `sdk/collector/collector.go`
- Create: `sdk/finding/finding.go`
- Create: `sdk/adapter/target.go`
- Create: `sdk/adapter/security.go`
- Create: `sdk/adapter/adapter.go`
- Create: `internal/platform/adapterregistry/registry.go`
- Create: `internal/platform/adapterregistry/registry_test.go`

**Interfaces:**
- Produces: `collector.Collector`, `finding.Rule`, `adapter.Adapter`, `adapter.Runtime`, `adapter.ParseTarget`, `adapterregistry.Registry`.

- [ ] **Step 1: Add collector and finding contracts**

Create `sdk/collector/collector.go`:

```go
package collector

import (
    "context"
    "time"
    "github.com/kefyusuf/dbprobe/sdk/capability"
    "github.com/kefyusuf/dbprobe/sdk/signal"
)

type Strategy string
type Phase string

const (
    StrategySnapshot Strategy = "snapshot"
    StrategyCounter Strategy = "counter"
    PhaseSingle Phase = "single"
    PhaseSampleA Phase = "sample_a"
    PhaseSampleB Phase = "sample_b"
)

type Descriptor struct {
    ID string
    Requires []capability.Capability
    Produces []signal.Key
    Strategy Strategy
}

type Request struct {
    Phase Phase
    CollectedAt time.Time
}

type Collector interface {
    Descriptor() Descriptor
    Collect(context.Context, Request) ([]signal.Observation, error)
}
```

Create `sdk/finding/finding.go`:

```go
package finding

import (
    "github.com/kefyusuf/dbprobe/sdk/capability"
    "github.com/kefyusuf/dbprobe/sdk/object"
    "github.com/kefyusuf/dbprobe/sdk/signal"
)

type ID string
type Severity string

type Finding struct {
    ID ID `json:"id"`
    Title string `json:"title"`
    Severity Severity `json:"severity"`
    Object object.Ref `json:"object"`
    Evidence []signal.Observation `json:"evidence,omitempty"`
    Summary string `json:"summary"`
    Guidance string `json:"guidance,omitempty"`
    Confidence float64 `json:"confidence"`
}

type AnalysisContext struct {
    Capabilities capability.Set
    Current []signal.Observation
    Previous []signal.Observation
    Deltas []signal.Delta
}

type Rule interface {
    ID() ID
    Requires() []capability.Capability
    Evaluate(AnalysisContext) []Finding
}
```

- [ ] **Step 2: Add adapter contracts**

Create `sdk/adapter/target.go`:

```go
package adapter

import (
    "fmt"
    "net/url"
)

type TargetSpec struct { RawURL, Scheme string }

type TargetMetadata struct {
    Engine string `json:"engine"`
    AdapterID string `json:"adapter_id"`
    Fingerprint string `json:"fingerprint"`
    DisplayName string `json:"display_name"`
}

func ParseTarget(raw string) (TargetSpec, error) {
    u, err := url.Parse(raw)
    if err != nil || u.Scheme == "" {
        return TargetSpec{}, fmt.Errorf("invalid target URL")
    }
    return TargetSpec{RawURL: raw, Scheme: u.Scheme}, nil
}
```

Create `sdk/adapter/security.go`:

```go
package adapter

type Privilege struct {
    Name string `json:"name"`
    Scope string `json:"scope,omitempty"`
    Reason string `json:"reason,omitempty"`
}

type SecurityProfile struct {
    ReadOnlyGuaranteed bool `json:"read_only_guaranteed"`
    Required []Privilege `json:"required,omitempty"`
    Recommended []Privilege `json:"recommended,omitempty"`
    Missing []Privilege `json:"missing,omitempty"`
}
```

Create `sdk/adapter/adapter.go`:

```go
package adapter

import (
    "context"
    "github.com/kefyusuf/dbprobe/sdk/capability"
    "github.com/kefyusuf/dbprobe/sdk/collector"
    "github.com/kefyusuf/dbprobe/sdk/finding"
)

const ContractVersion = "v0.1"

type Metadata struct { ID, Name, Version, ContractVersion string }
type OpenOptions struct{}

type Adapter interface {
    Metadata() Metadata
    Match(TargetSpec) bool
    Open(context.Context, TargetSpec, OpenOptions) (Runtime, error)
}

type Runtime interface {
    Target() TargetMetadata
    Capabilities() capability.Set
    Collectors() []collector.Collector
    Rules() []finding.Rule
    SecurityProfile() SecurityProfile
    Close() error
}
```

- [ ] **Step 3: Write registry tests with concrete test doubles**

Create `internal/platform/adapterregistry/registry_test.go`:

```go
package adapterregistry_test

import (
    "context"
    "testing"

    "github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
    "github.com/kefyusuf/dbprobe/sdk/adapter"
)

type stubAdapter struct{ id, scheme, contract string }

func (s stubAdapter) Metadata() adapter.Metadata {
    return adapter.Metadata{ID: s.id, Name: s.id, Version: "test", ContractVersion: s.contract}
}
func (s stubAdapter) Match(spec adapter.TargetSpec) bool { return spec.Scheme == s.scheme }
func (s stubAdapter) Open(context.Context, adapter.TargetSpec, adapter.OpenOptions) (adapter.Runtime, error) { return nil, nil }

func TestResolveReturnsMatchingAdapter(t *testing.T) {
    r, err := adapterregistry.New(stubAdapter{"fake", "fake", adapter.ContractVersion})
    if err != nil { t.Fatal(err) }
    spec, err := adapter.ParseTarget("fake://local")
    if err != nil { t.Fatal(err) }
    got, err := r.Resolve(spec)
    if err != nil { t.Fatal(err) }
    if got.Metadata().ID != "fake" { t.Fatalf("got %q", got.Metadata().ID) }
}

func TestResolveFailsWhenNoAdapterMatches(t *testing.T) {
    r, err := adapterregistry.New(stubAdapter{"fake", "fake", adapter.ContractVersion})
    if err != nil { t.Fatal(err) }
    spec, _ := adapter.ParseTarget("redis://local")
    if _, err := r.Resolve(spec); err == nil { t.Fatal("expected no-match error") }
}

func TestNewRejectsDuplicateIDs(t *testing.T) {
    _, err := adapterregistry.New(
        stubAdapter{"same", "a", adapter.ContractVersion},
        stubAdapter{"same", "b", adapter.ContractVersion},
    )
    if err == nil { t.Fatal("expected duplicate-ID error") }
}

func TestNewRejectsContractMismatch(t *testing.T) {
    _, err := adapterregistry.New(stubAdapter{"fake", "fake", "v999"})
    if err == nil { t.Fatal("expected contract-version error") }
}
```

Run:

```bash
go test ./internal/platform/adapterregistry -v
```

Expected: FAIL.

- [ ] **Step 4: Implement registry**

Create `internal/platform/adapterregistry/registry.go` with exactly these exported methods:

```go
type Registry struct { adapters []adapter.Adapter }
func New(adapters ...adapter.Adapter) (*Registry, error)
func (r *Registry) Resolve(spec adapter.TargetSpec) (adapter.Adapter, error)
```

`New` validates non-empty unique adapter IDs and exact `adapter.ContractVersion`. `Resolve` returns the first matcher. Errors may contain adapter ID and target scheme, but never `TargetSpec.RawURL`.

Run:

```bash
go test ./internal/platform/adapterregistry -v
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add sdk/adapter sdk/collector sdk/finding internal/platform/adapterregistry
git commit -m "feat: define adapter runtime contract and registry"
```

---

### Task 4: Capability-aware collection planner and reset-safe deltas

**Files:**
- Create: `internal/core/collection/planner.go`
- Create: `internal/core/collection/planner_test.go`

**Interfaces:**
- Produces: `collection.New`, `collection.RealWaiter`, `Planner.Run`, `collection.Result`, `collection.Warning`.

- [ ] **Step 1: Write four planner tests**

Create test collectors in `planner_test.go` that implement the SDK collector interface and cover:

```go
func TestPlannerSkipsCollectorMissingCapability(t *testing.T)
func TestPlannerPreservesSuccessfulEvidenceWhenCollectorFails(t *testing.T)
func TestPlannerSamplesCounterTwiceAndComputesRate(t *testing.T)
func TestPlannerTreatsCounterDecreaseAsReset(t *testing.T)
```

Use a `noopWaiter`:

```go
type noopWaiter struct{}
func (noopWaiter) Wait(context.Context, time.Duration) error { return nil }
```

For the rate test, sample A must return `100`, sample B `140`, window `10*time.Second`; assert delta `40` and rate `4`.

For reset, sample A `100`, sample B `5`; assert no negative delta is emitted and a warning identifies the reset.

Run `go test ./internal/core/collection -v`. Expected: FAIL.

- [ ] **Step 2: Implement planner public contract**

Create `internal/core/collection/planner.go`:

```go
package collection

import (
    "context"
    "fmt"
    "sort"
    "time"

    "github.com/kefyusuf/dbprobe/sdk/capability"
    "github.com/kefyusuf/dbprobe/sdk/collector"
    "github.com/kefyusuf/dbprobe/sdk/signal"
)

type Warning struct {
    CollectorID string `json:"collector_id"`
    Reason string `json:"reason"`
}

type Result struct {
    Observations []signal.Observation `json:"observations"`
    Deltas []signal.Delta `json:"deltas,omitempty"`
    Warnings []Warning `json:"warnings,omitempty"`
}

type Waiter interface { Wait(context.Context, time.Duration) error }

type RealWaiter struct{}
func (RealWaiter) Wait(ctx context.Context, d time.Duration) error {
    timer := time.NewTimer(d)
    defer timer.Stop()
    select {
    case <-ctx.Done(): return ctx.Err()
    case <-timer.C: return nil
    }
}

type Planner struct { waiter Waiter; now func() time.Time }
func New(waiter Waiter, now func() time.Time) *Planner { return &Planner{waiter: waiter, now: now} }
```

Implement `Run` with this algorithm:

```text
eligible = collectors whose Requires are all present
snapshot collectors: collect PhaseSingle once
counter collectors: collect PhaseSampleA
if counters exist: waiter.Wait(sampleWindow)
counter collectors: collect PhaseSampleB
append B observations as current observations
match numeric A/B by key + object.kind + object.id
if B >= A: emit sampled Delta with (B-A)/windowSeconds
if B < A: emit reset warning and no numeric delta
collector error: append warning and continue other collectors
sort observations/deltas deterministically before returning
```

The helper key for A/B matching is:

```go
func identity(o signal.Observation) string {
    return fmt.Sprintf("%s|%s|%s", o.Key, o.Object.Kind, o.Object.ID)
}
```

Stable sort key is the same identity string.

- [ ] **Step 3: Verify planner**

Run:

```bash
go test ./internal/core/collection -v
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/core/collection
git commit -m "feat: add capability-aware collection planner"
```

---

### Task 5: Fake adapter and reusable adapter contract suite

**Files:**
- Create: `adapters/fake/adapter.go`
- Create: `adapters/fake/runtime.go`
- Create: `adapters/fake/collectors.go`
- Create: `adapters/fake/adapter_test.go`
- Create: `test/contract/adapter_contract_test.go`

**Interfaces:**
- Produces: `fake.New() adapter.Adapter`; deterministic target `fake://local`.

- [ ] **Step 1: Write fake adapter tests**

Tests must assert these exact values:

```text
adapter ID: fake
adapter contract: v0.1
engine: fake
fingerprint: stable for fake://local
capabilities: activity.sessions, workload.query_summary
security.read_only_guaranteed: true
collector IDs: fake.health, fake.workload
```

Run `go test ./adapters/fake -v`. Expected: FAIL.

- [ ] **Step 2: Implement fake adapter/runtime**

`fake.New()` returns a stateless adapter matching only scheme `fake`.

Target fingerprint implementation:

```go
sum := sha256.Sum256([]byte("fake|local"))
fingerprint := hex.EncodeToString(sum[:8])
```

Runtime capabilities:

```go
capability.New("activity.sessions", "workload.query_summary")
```

Security profile:

```go
adapter.SecurityProfile{ReadOnlyGuaranteed: true}
```

`Rules()` returns `[]finding.Rule{}` and `Close()` returns `nil` on every call.

- [ ] **Step 3: Implement deterministic collectors**

`fake.health` is `StrategySnapshot`, requires `activity.sessions`, and returns:

```text
core.connections.used = 12 count
core.connections.limit = 100 count
```

`fake.workload` is `StrategyCounter`, requires `workload.query_summary`, and returns:

```text
first call  -> core.query.calls = 100 count
second call -> core.query.calls = 140 count
```

Both use:

```go
object.Ref{Kind: "fake.instance", ID: "local"}
```

The workload collector stores its call count behind `sync.Mutex`.

- [ ] **Step 4: Add reusable contract suite**

Create `test/contract/adapter_contract_test.go` with a table initially containing only `fake.New()`. For each adapter verify:

```text
metadata ID/name/version/contract are non-empty
contract version == adapter.ContractVersion
matching test target opens successfully
target fingerprint is non-empty and stable across two opens
collector IDs are unique and non-empty
produced signal keys are non-empty
required capabilities are non-empty strings
Close() may be called twice without failure
```

Use test target `fake://local` for the fake entry. The table structure must allow MySQL and Mongo entries to be added without changing the assertions.

- [ ] **Step 5: Verify with race detector**

```bash
go test -race ./adapters/fake ./test/contract -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add adapters/fake test/contract
git commit -m "feat: add fake adapter and contract test harness"
```

---

### Task 6: Inspect application service and `v1alpha1` report

**Files:**
- Create: `internal/app/inspect/service.go`
- Create: `internal/app/inspect/service_test.go`

**Interfaces:**
- Consumes: registry, adapter runtime, collection planner.
- Produces: `inspect.Service.Run(context.Context, string, time.Duration) (inspect.Report, error)`.

- [ ] **Step 1: Write the end-to-end service test**

Compose registry with `fake.New()`, planner with `noopWaiter`, and call:

```go
report, err := service.Run(context.Background(), "fake://local", 10*time.Second)
```

Assert:

```text
schema_version = dbprobe.inspect/v1alpha1
target.engine = fake
target.adapter_id = fake
capabilities sorted exactly [activity.sessions, workload.query_summary]
observations include used=12, limit=100, calls=140
deltas include core.query.calls delta=40 rate=4
findings length = 0
security.read_only_guaranteed = true
warnings length = 0
```

Run `go test ./internal/app/inspect -v`. Expected: FAIL.

- [ ] **Step 2: Implement report**

Create in `service.go`:

```go
const SchemaVersion = "dbprobe.inspect/v1alpha1"

type Report struct {
    SchemaVersion string `json:"schema_version"`
    CollectedAt time.Time `json:"collected_at"`
    Target adapter.TargetMetadata `json:"target"`
    Capabilities []capability.Capability `json:"capabilities"`
    Security adapter.SecurityProfile `json:"security"`
    Observations []signal.Observation `json:"observations"`
    Deltas []signal.Delta `json:"deltas"`
    Findings []finding.Finding `json:"findings"`
    Warnings []collection.Warning `json:"warnings"`
}
```

`Service.Run` order is mandatory:

```text
ParseTarget
Resolve adapter
Open runtime
defer Close
read Target/Capabilities/SecurityProfile
Planner.Run
build finding.AnalysisContext
evaluate only rules whose Requires are present
return initialized non-nil Findings/Deltas/Warnings slices
```

Returned error messages must not interpolate `TargetSpec.RawURL`.

- [ ] **Step 3: Verify service and full suite**

```bash
go test ./internal/app/inspect -v
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/app/inspect
git commit -m "feat: add inspect application use case"
```

---

### Task 7: JSON and terminal surfaces

**Files:**
- Create: `internal/surfaces/json/render.go`
- Create: `internal/surfaces/json/render_test.go`
- Create: `internal/surfaces/terminal/render.go`
- Create: `internal/surfaces/terminal/render_test.go`

**Interfaces:**
- Produces: `jsonsurface.Render(io.Writer, inspect.Report) error`, `terminal.Render(io.Writer, inspect.Report) error`.

- [ ] **Step 1: Write JSON contract test**

Render a fixed `inspect.Report`, decode it into `map[string]any`, and assert the presence of exactly these top-level contract fields:

```text
schema_version
collected_at
target
capabilities
security
observations
deltas
findings
warnings
```

Assert `schema_version == dbprobe.inspect/v1alpha1` and empty findings serialize as `[]`, not `null`.

Run `go test ./internal/surfaces/json -v`. Expected: FAIL.

- [ ] **Step 2: Implement JSON renderer**

```go
func Render(w io.Writer, report inspect.Report) error {
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return enc.Encode(report)
}
```

No second JSON DTO is introduced in this plan.

- [ ] **Step 3: Write and implement terminal renderer**

The test must require these lines for the fake report:

```text
dbprobe · fake · local
read-only: yes
capabilities: 2
observations: 3
deltas: 1
findings: 0
```

Implement with `fmt.Fprintf` only; do not add color/styling dependency yet.

- [ ] **Step 4: Verify surfaces**

```bash
go test ./internal/surfaces/... -v
go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/surfaces
git commit -m "feat: add JSON and terminal report surfaces"
```

---

### Task 8: Cobra CLI composition root and executable inspect command

**Files:**
- Modify: `go.mod`
- Create/modify: `go.sum`
- Create: `cmd/dbprobe/main.go`
- Create: `cmd/dbprobe/root.go`
- Create: `cmd/dbprobe/inspect.go`

**Interfaces:**
- Produces: `dbprobe inspect <target> --format=text|json --sample-window=<duration>`.

- [ ] **Step 1: Add Cobra only now that code consumes it**

Run:

```bash
go get github.com/spf13/cobra@v1.10.2
go mod tidy
```

Expected: Cobra remains in `go.mod` because Task 8 immediately imports it; `go.sum` is created.

- [ ] **Step 2: Implement root command**

`root.go` exposes:

```go
func newRootCommand() *cobra.Command {
    cmd := &cobra.Command{Use: "dbprobe", SilenceUsage: true}
    cmd.AddCommand(newInspectCommand())
    return cmd
}
```

`main.go` executes the root command and uses `os.Exit(1)` on error without a stack trace.

- [ ] **Step 3: Implement inspect command composition**

`newInspectCommand()` must:

- require exactly one target argument;
- default `--format` to `text`;
- allow only `text` and `json`;
- default `--sample-window` to `1s`;
- build registry with `fake.New()`;
- use `collection.New(collection.RealWaiter{}, time.Now)`;
- call inspect service;
- dispatch the returned report to terminal or JSON renderer.

The concrete `fake` adapter import is allowed here because `cmd/dbprobe` is the composition root.

- [ ] **Step 4: Build and run JSON smoke test**

```bash
mkdir -p bin
go build -o ./bin/dbprobe ./cmd/dbprobe
./bin/dbprobe inspect fake://local --format=json --sample-window=10ms
```

Expected JSON contains `schema_version: dbprobe.inspect/v1alpha1`, target engine `fake`, and `core.query.calls` delta `40`. For a `10ms` window the rate is approximately `4000` per second.

- [ ] **Step 5: Run text and failure smoke tests**

```bash
./bin/dbprobe inspect fake://local --sample-window=10ms
./bin/dbprobe inspect redis://local --sample-window=10ms
./bin/dbprobe inspect fake://local --format=xml --sample-window=10ms
```

Expected:

- fake text command exits `0` and prints `dbprobe · fake · local`;
- redis command exits non-zero with safe scheme-level error;
- XML command exits non-zero before adapter open;
- no command prints a stack trace or credentials.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum cmd/dbprobe
git commit -m "feat: expose inspect vertical slice through CLI"
```

---

### Task 9: Foundation acceptance gate

**Files:**
- Modify only files from Tasks 1-8 if verification exposes a concrete defect.

**Interfaces:**
- Produces: evidence that MySQL adapter work can begin without changing dependency direction.

- [ ] **Step 1: Format, vet, and standard tests**

```bash
make fmt
make fmt-check
make vet
make test
```

Expected: PASS.

- [ ] **Step 2: Race suite**

```bash
make race
```

Expected: PASS with no race reports.

- [ ] **Step 3: Explicit architecture test**

```bash
go test ./test/architecture -run TestDependencyBoundaries -v
```

Expected: PASS.

- [ ] **Step 4: Executable contract check**

```bash
go build -o ./bin/dbprobe ./cmd/dbprobe
./bin/dbprobe inspect fake://local --format=json --sample-window=10ms > /tmp/dbprobe-report.json
./bin/dbprobe inspect fake://local --sample-window=10ms
```

Use the standard library to validate the JSON without adding a project dependency:

```bash
go run ./cmd/dbprobe inspect fake://local --format=json --sample-window=1ms > /tmp/dbprobe-report-2.json
```

Both JSON files must parse, carry `dbprobe.inspect/v1alpha1`, and contain the same signal keys.

- [ ] **Step 5: Confirm all exit criteria**

Every statement must be true:

```text
[ ] core imports no real database driver or concrete adapter
[ ] fake:// resolves through the registry
[ ] missing capabilities skip incompatible collectors
[ ] collector failure preserves other evidence and produces a warning
[ ] counters are sampled centrally by core
[ ] counter resets never create negative rates
[ ] security profile explicitly states read-only status
[ ] JSON and terminal consume the same inspect.Report
[ ] adapter contract suite can accept later MySQL/Mongo test cases
[ ] architecture guard runs in GitHub Actions
[ ] go test -race ./... passes
```

- [ ] **Step 6: Commit only real verification fixes**

If verification changed code:

```bash
git add <only-the-corrected-paths>
git commit -m "fix: satisfy foundation runtime acceptance gate"
```

If verification required no change, create no commit.

---

## Definition of Done

A fresh checkout must successfully run:

```bash
make ci
make race
go build -o ./bin/dbprobe ./cmd/dbprobe
./bin/dbprobe inspect fake://local --format=json --sample-window=10ms
```

The report must be `dbprobe.inspect/v1alpha1`, contain deterministic fake observations, expose the read-only security profile, and contain a centrally computed `core.query.calls` delta.

Only after this gate passes should the next plan start: **MySQL Adapter MVP** covering MySQL 8.0/8.4 connectivity, privilege/capability discovery, Performance Schema/sys collection, and initial engine-native deterministic findings without modifying core dependency direction.
