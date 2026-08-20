# dbprobe Foundation Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first working vertical slice of dbprobe: a database-agnostic runtime that resolves an adapter, discovers capabilities, executes snapshot/counter collectors, computes reset-aware deltas, and renders a versioned report through CLI/JSON using a fake adapter.

**Architecture:** The implementation preserves the Core/SDK/Adapter boundaries defined by the architecture spec. No real database driver is introduced in this plan. A deterministic fake adapter is the executable proof that adapter contracts, capability filtering, centralized sampling, partial-failure behavior, and machine-readable output work before MySQL is allowed into the codebase.

**Tech Stack:** Go 1.25+, Cobra CLI, Go standard library tests, GitHub Actions. No database driver, SQLite, MCP, LLM, or external plugin mechanism in this plan.

**Spec:** `docs/superpowers/specs/2026-08-21-dbprobe-v0.1-architecture-design.md`

## Global Constraints

- Core is database-agnostic; `internal/core` must not import database-engine packages or drivers.
- SDK packages must not import concrete adapters.
- Adapters must not import other adapters.
- Surfaces must not directly depend on concrete adapter packages.
- Diagnostics are deterministic; no LLM participates in collection or finding generation.
- Inspection is read-only by contract; fake adapter reports a read-only security profile.
- Missing capabilities and collector failures must degrade a section, not silently fabricate completeness.
- Engine-native concepts stay namespaced; portable concepts use `core.*` only when semantics are genuinely portable.
- JSON output is a versioned machine contract from the first executable slice.
- Counter timing/delta calculation belongs to core, not individual adapters.
- SDK is pre-1.0 and may change until the MongoDB abstraction-validation gate is passed.
- Commits must be small and reviewable; stage only paths owned by the current task.

---

## File Structure Locked By This Plan

```text
dbprobe/
├── .github/
│   └── workflows/
│       └── ci.yml
├── cmd/
│   └── dbprobe/
│       ├── main.go
│       ├── root.go
│       └── inspect.go
├── adapters/
│   └── fake/
│       ├── adapter.go
│       ├── runtime.go
│       ├── collectors.go
│       └── adapter_test.go
├── internal/
│   ├── app/
│   │   └── inspect/
│   │       ├── service.go
│   │       └── service_test.go
│   ├── core/
│   │   └── collection/
│   │       ├── planner.go
│   │       └── planner_test.go
│   ├── platform/
│   │   └── adapterregistry/
│   │       ├── registry.go
│   │       └── registry_test.go
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
│   ├── collector/
│   │   └── collector.go
│   ├── finding/
│   │   └── finding.go
│   ├── object/
│   │   └── ref.go
│   └── signal/
│       ├── observation.go
│       ├── delta.go
│       └── observation_test.go
├── test/
│   ├── architecture/
│   │   └── dependencies_test.go
│   └── contract/
│       └── adapter_contract_test.go
├── go.mod
├── go.sum
└── Makefile
```

Responsibility rules:

- `sdk/*` defines contracts concrete adapters may consume or implement.
- `internal/core/*` performs engine-agnostic orchestration/analysis.
- `internal/platform/*` contains infrastructure wiring such as adapter registration.
- `internal/app/*` implements use-cases and produces application reports.
- `internal/surfaces/*` renders reports; it does not inspect databases.
- `adapters/*` owns engine-specific connectivity and collection.
- `cmd/dbprobe/*` is the composition root and command parsing only.

---

### Task 1: Repository engineering foundation and dependency boundary guard

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `.github/workflows/ci.yml`
- Create: `test/architecture/dependencies_test.go`

**Interfaces:**
- Consumes: none.
- Produces: module path `github.com/kefyusuf/dbprobe`; CI commands `make fmt-check`, `make vet`, `make test`; architecture import guard used by all later tasks.

- [ ] **Step 1: Initialize the Go module and pin Cobra**

Create `go.mod` with:

```go
module github.com/kefyusuf/dbprobe

go 1.25

require github.com/spf13/cobra v1.10.2
```

Run:

```bash
go mod tidy
```

Expected: `go.sum` is generated and the command exits `0`.

- [ ] **Step 2: Add the architecture import test before implementation packages exist**

Create `test/architecture/dependencies_test.go`:

```go
package architecture_test

import (
    "go/ast"
    "go/parser"
    "go/token"
    "io/fs"
    "path/filepath"
    "strconv"
    "strings"
    "testing"
)

func TestDependencyBoundaries(t *testing.T) {
    rules := []struct {
        root      string
        forbidden []string
    }{
        {"../../internal/core", []string{"/adapters/", "go-sql-driver/mysql", "jackc/pgx", "mongo-driver", "gocql"}},
        {"../../sdk", []string{"/adapters/"}},
        {"../../internal/surfaces", []string{"/adapters/"}},
    }

    for _, rule := range rules {
        err := filepath.WalkDir(rule.root, func(path string, d fs.DirEntry, err error) error {
            if err != nil {
                return err
            }
            if d.IsDir() || !strings.HasSuffix(path, ".go") {
                return nil
            }
            file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
            if err != nil {
                return err
            }
            for _, imp := range file.Imports {
                importPath, _ := strconv.Unquote(imp.Path.Value)
                for _, forbidden := range rule.forbidden {
                    if strings.Contains(importPath, forbidden) {
                        t.Errorf("%s imports forbidden dependency %q", path, importPath)
                    }
                }
            }
            return nil
        })
        if err != nil && !strings.Contains(err.Error(), "no such file or directory") {
            t.Fatal(err)
        }
    }
}

var _ ast.Node
```

The temporary `ast` reference prevents an unused import if the file is copied exactly; remove both the `go/ast` import and final line if your formatter/compiler reports they are unnecessary before committing.

- [ ] **Step 3: Add reproducible local commands**

Create `Makefile`:

```make
.PHONY: fmt fmt-check vet test ci

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

fmt-check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './vendor/*'))"

vet:
	go vet ./...

test:
	go test ./...

ci: fmt-check vet test
```

- [ ] **Step 4: Add GitHub Actions CI**

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

- [ ] **Step 5: Verify the foundation**

Run:

```bash
make ci
```

Expected: all commands exit `0`; `go test ./...` includes `test/architecture`.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum Makefile .github/workflows/ci.yml test/architecture/dependencies_test.go
git commit -m "chore: establish Go project and architecture guard"
```

---

### Task 2: Capability, object, and signal primitives

**Files:**
- Create: `sdk/capability/capability.go`
- Create: `sdk/capability/capability_test.go`
- Create: `sdk/object/ref.go`
- Create: `sdk/signal/observation.go`
- Create: `sdk/signal/delta.go`
- Create: `sdk/signal/observation_test.go`

**Interfaces:**
- Consumes: standard library only.
- Produces: `capability.Capability`, `capability.Set`, `object.Ref`, `signal.Observation`, `signal.Delta`, exactness/unit/sensitivity enums, numeric observation helpers.

- [ ] **Step 1: Write failing capability-set tests**

Create `sdk/capability/capability_test.go`:

```go
package capability_test

import (
    "reflect"
    "testing"

    "github.com/kefyusuf/dbprobe/sdk/capability"
)

func TestSetHasAndListIsStable(t *testing.T) {
    set := capability.New("storage.cache", "activity.sessions", "storage.cache")

    if !set.Has("activity.sessions") {
        t.Fatal("expected activity.sessions capability")
    }

    want := []capability.Capability{"activity.sessions", "storage.cache"}
    if got := set.List(); !reflect.DeepEqual(got, want) {
        t.Fatalf("List() = %#v, want %#v", got, want)
    }
}
```

Run:

```bash
go test ./sdk/capability -run TestSetHasAndListIsStable -v
```

Expected: FAIL because package/types do not exist.

- [ ] **Step 2: Implement the capability set**

Create `sdk/capability/capability.go`:

```go
package capability

import "sort"

type Capability string

type Set struct {
    values map[Capability]struct{}
}

func New(values ...Capability) Set {
    out := Set{values: make(map[Capability]struct{}, len(values))}
    for _, value := range values {
        out.values[value] = struct{}{}
    }
    return out
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

Run the capability test again. Expected: PASS.

- [ ] **Step 3: Write failing observation tests**

Create `sdk/signal/observation_test.go`:

```go
package signal_test

import (
    "testing"
    "time"

    "github.com/kefyusuf/dbprobe/sdk/object"
    "github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestNumberObservationRoundTrip(t *testing.T) {
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

Run:

```bash
go test ./sdk/signal -run TestNumberObservationRoundTrip -v
```

Expected: FAIL because signal/object types do not exist.

- [ ] **Step 4: Implement object identity and observation contracts**

Create `sdk/object/ref.go`:

```go
package object

type Ref struct {
    Kind string `json:"kind"`
    ID   string `json:"id"`
}
```

Create `sdk/signal/observation.go` with these public contracts:

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
    UnitCount        Unit = "count"
    UnitMilliseconds Unit = "milliseconds"
    UnitBytes        Unit = "bytes"
    UnitRatio        Unit = "ratio"
    UnitSeconds      Unit = "seconds"

    ExactnessScraped    Exactness = "scraped"
    ExactnessCumulative Exactness = "cumulative"
    ExactnessSampled    Exactness = "sampled"
    ExactnessEstimated  Exactness = "estimated"
    ExactnessUnavailable Exactness = "unavailable"
    ExactnessReset      Exactness = "reset"

    SensitivityMetadata   Sensitivity = "metadata"
    SensitivityQueryShape Sensitivity = "query_shape"
    SensitivityQueryText  Sensitivity = "query_text"
)

type Observation struct {
    Key         Key                 `json:"key"`
    Object      object.Ref          `json:"object"`
    Unit        Unit                `json:"unit"`
    Exactness   Exactness           `json:"exactness"`
    Number      *float64            `json:"number,omitempty"`
    Text        *string             `json:"text,omitempty"`
    Boolean     *bool               `json:"boolean,omitempty"`
    CollectedAt time.Time           `json:"collected_at"`
    Sensitivity Sensitivity         `json:"sensitivity"`
    Source      string              `json:"source,omitempty"`
    Reason      string              `json:"reason,omitempty"`
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

import (
    "time"

    "github.com/kefyusuf/dbprobe/sdk/object"
)

type Delta struct {
    Key           Key        `json:"key"`
    Object        object.Ref `json:"object"`
    Unit          Unit       `json:"unit"`
    Delta         float64    `json:"delta"`
    RatePerSecond float64    `json:"rate_per_second"`
    Window        time.Duration `json:"-"`
    WindowSeconds float64    `json:"window_seconds"`
    Exactness     Exactness  `json:"exactness"`
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

### Task 3: Adapter/runtime contracts and compile-time registry

**Files:**
- Create: `sdk/collector/collector.go`
- Create: `sdk/finding/finding.go`
- Create: `sdk/adapter/target.go`
- Create: `sdk/adapter/security.go`
- Create: `sdk/adapter/adapter.go`
- Create: `internal/platform/adapterregistry/registry.go`
- Create: `internal/platform/adapterregistry/registry_test.go`

**Interfaces:**
- Consumes: Task 2 primitives.
- Produces: `adapter.Adapter`, `adapter.Runtime`, `adapter.ParseTarget`, `collector.Collector`, `finding.Rule`, `adapterregistry.Registry.Resolve`.

- [ ] **Step 1: Define collector and finding contracts**

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
    StrategyCounter  Strategy = "counter"

    PhaseSingle  Phase = "single"
    PhaseSampleA Phase = "sample_a"
    PhaseSampleB Phase = "sample_b"
)

type Descriptor struct {
    ID       string
    Requires []capability.Capability
    Produces []signal.Key
    Strategy Strategy
}

type Request struct {
    Phase       Phase
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
    ID         ID                   `json:"id"`
    Title      string               `json:"title"`
    Severity   Severity             `json:"severity"`
    Object     object.Ref           `json:"object"`
    Evidence   []signal.Observation `json:"evidence,omitempty"`
    Summary    string               `json:"summary"`
    Guidance   string               `json:"guidance,omitempty"`
    Confidence float64              `json:"confidence"`
}

type AnalysisContext struct {
    Capabilities capability.Set
    Current      []signal.Observation
    Previous     []signal.Observation
    Deltas       []signal.Delta
}

type Rule interface {
    ID() ID
    Requires() []capability.Capability
    Evaluate(AnalysisContext) []Finding
}
```

- [ ] **Step 2: Define target, security, adapter, and runtime contracts**

Create `sdk/adapter/target.go` with `TargetSpec`, `TargetMetadata`, and parser:

```go
package adapter

import (
    "fmt"
    "net/url"
)

type TargetSpec struct {
    RawURL string
    Scheme string
}

type TargetMetadata struct {
    Engine      string `json:"engine"`
    AdapterID   string `json:"adapter_id"`
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
    Name   string `json:"name"`
    Scope  string `json:"scope,omitempty"`
    Reason string `json:"reason,omitempty"`
}

type SecurityProfile struct {
    ReadOnlyGuaranteed bool        `json:"read_only_guaranteed"`
    Required           []Privilege `json:"required,omitempty"`
    Recommended        []Privilege `json:"recommended,omitempty"`
    Missing            []Privilege `json:"missing,omitempty"`
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

type Metadata struct {
    ID              string
    Name            string
    Version         string
    ContractVersion string
}

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

- [ ] **Step 3: Write failing registry tests**

Create `internal/platform/adapterregistry/registry_test.go` with a minimal stub adapter and verify:

```go
func TestResolveReturnsMatchingAdapter(t *testing.T) { /* fake mysql:// matcher, expect returned metadata ID */ }
func TestResolveFailsWhenNoAdapterMatches(t *testing.T) { /* redis:// target, expect error */ }
func TestNewRejectsDuplicateAdapterIDs(t *testing.T) { /* two adapters with ID x, expect error */ }
```

Use real test doubles implementing the exact `adapter.Adapter` interface; do not weaken the interface for tests.

Run:

```bash
go test ./internal/platform/adapterregistry -v
```

Expected: FAIL because registry is missing.

- [ ] **Step 4: Implement registry**

Create `internal/platform/adapterregistry/registry.go` exposing:

```go
type Registry struct { adapters []adapter.Adapter }

func New(adapters ...adapter.Adapter) (*Registry, error)
func (r *Registry) Resolve(spec adapter.TargetSpec) (adapter.Adapter, error)
```

Rules:

- adapter IDs must be non-empty and unique;
- adapters with a `ContractVersion` different from `adapter.ContractVersion` are rejected;
- `Resolve` returns the first matching adapter;
- no match returns an error containing the target scheme but never credentials/raw URL.

Run:

```bash
go test ./internal/platform/adapterregistry -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add sdk/adapter sdk/collector sdk/finding internal/platform/adapterregistry
git commit -m "feat: define adapter runtime contract and registry"
```

---

### Task 4: Capability-aware collection planner with centralized counter sampling

**Files:**
- Create: `internal/core/collection/planner.go`
- Create: `internal/core/collection/planner_test.go`

**Interfaces:**
- Consumes: `capability.Set`, `collector.Collector`, `signal.Observation`, `signal.Delta`.
- Produces: `collection.Planner.Run(ctx, capabilities, collectors, sampleWindow) collection.Result`.

- [ ] **Step 1: Write failing planner tests**

The tests must cover four behaviors with explicit fake collectors:

```go
func TestPlannerSkipsCollectorWithMissingCapability(t *testing.T)
func TestPlannerKeepsSuccessfulCollectorsWhenOneFails(t *testing.T)
func TestPlannerSamplesCounterTwiceAndComputesRate(t *testing.T)
func TestPlannerMarksCounterResetInsteadOfProducingNegativeDelta(t *testing.T)
```

For the rate test, sample A returns `100`, sample B returns `140`, sample window is `10s`; expect delta `40` and rate `4` per second.

For the reset test, sample A returns `100`, sample B returns `5`; expect no negative rate and one warning/reset marker with `ExactnessReset`.

Run:

```bash
go test ./internal/core/collection -v
```

Expected: FAIL because planner does not exist.

- [ ] **Step 2: Implement deterministic planner contracts**

Create `internal/core/collection/planner.go` around:

```go
type Warning struct {
    CollectorID string `json:"collector_id"`
    Reason      string `json:"reason"`
}

type Result struct {
    Observations []signal.Observation `json:"observations"`
    Deltas       []signal.Delta       `json:"deltas,omitempty"`
    Warnings     []Warning            `json:"warnings,omitempty"`
}

type Waiter interface {
    Wait(context.Context, time.Duration) error
}

type Planner struct {
    waiter Waiter
    now    func() time.Time
}

func New(waiter Waiter, now func() time.Time) *Planner
func (p *Planner) Run(ctx context.Context, caps capability.Set, collectors []collector.Collector, sampleWindow time.Duration) Result
```

Implementation rules:

1. unsupported collectors are skipped without invoking them;
2. snapshot collectors run once with `PhaseSingle`;
3. counter collectors run once with `PhaseSampleA`;
4. planner waits exactly `sampleWindow` once when at least one counter collector exists;
5. counter collectors run again with `PhaseSampleB`;
6. only matching numeric `(signal key, object kind, object ID)` values are delta-calculated;
7. B < A is treated as reset, never a negative rate;
8. collector errors append warnings and preserve successful results from other collectors;
9. returned observation/delta ordering is stable: collector order, then observation key/object order.

Provide a real waiter implementation in the same file:

```go
type RealWaiter struct{}
func (RealWaiter) Wait(ctx context.Context, d time.Duration) error
```

Use a timer/select so context cancellation aborts the wait.

- [ ] **Step 3: Run focused tests and full suite**

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

### Task 5: Fake adapter and reusable adapter contract tests

**Files:**
- Create: `adapters/fake/adapter.go`
- Create: `adapters/fake/runtime.go`
- Create: `adapters/fake/collectors.go`
- Create: `adapters/fake/adapter_test.go`
- Create: `test/contract/adapter_contract_test.go`

**Interfaces:**
- Consumes: Tasks 2-4 contracts.
- Produces: `fake.New() adapter.Adapter`; stable target `fake://local`; deterministic snapshot/counter collectors used by e2e tests.

- [ ] **Step 1: Write fake-adapter contract expectations**

In `adapters/fake/adapter_test.go`, verify:

- metadata ID is `fake`;
- contract version equals `adapter.ContractVersion`;
- `Match` accepts `fake://local` and rejects `mysql://local`;
- runtime target engine is `fake`;
- runtime fingerprint is stable across two opens of the same target;
- security profile reports `ReadOnlyGuaranteed=true`;
- capabilities include `activity.sessions` and `workload.query_summary`.

Run:

```bash
go test ./adapters/fake -v
```

Expected: FAIL.

- [ ] **Step 2: Implement fake adapter**

`fake.New()` returns a stateless adapter. `Open` returns a runtime whose fingerprint is `sha256("fake|local")` truncated to 16 hex characters.

The runtime exposes two collectors:

1. `fake.health` (`StrategySnapshot`) producing:
   - `core.connections.used = 12 count`
   - `core.connections.limit = 100 count`
2. `fake.workload` (`StrategyCounter`) producing `core.query.calls`:
   - first invocation: `100`
   - second invocation: `140`

Both use object ref:

```go
object.Ref{Kind: "fake.instance", ID: "local"}
```

The counter collector keeps call state inside one runtime instance and is protected by a mutex so tests remain race-safe.

`Rules()` returns an empty slice in this plan.

- [ ] **Step 3: Add reusable adapter-contract assertions**

Create `test/contract/adapter_contract_test.go` as a black-box contract test that imports `adapters/fake` and validates generic invariants:

- adapter ID/name/version/contract version non-empty;
- target fingerprint non-empty and stable;
- all collector IDs are unique;
- all produced signal keys are non-empty;
- all finding-rule IDs, if any, are namespaced;
- `Close()` is idempotent for the fake runtime;
- no collector descriptor requires an empty capability.

This test is intentionally structured so later MySQL/Mongo adapters can be added as table entries without copying the invariant suite.

- [ ] **Step 4: Run race-enabled adapter tests**

```bash
go test -race ./adapters/fake ./test/contract -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add adapters/fake test/contract
git commit -m "feat: add fake adapter and contract test harness"
```

---

### Task 6: Inspect application use-case and versioned report

**Files:**
- Create: `internal/app/inspect/service.go`
- Create: `internal/app/inspect/service_test.go`

**Interfaces:**
- Consumes: adapter registry, adapter runtime, collection planner.
- Produces: `inspect.Service.Run(ctx, rawTarget, sampleWindow) (inspect.Report, error)` and JSON schema version `dbprobe.inspect/v1alpha1`.

- [ ] **Step 1: Write failing application test**

Build the service with a registry containing `fake.New()` and a planner with an instant test waiter. Call:

```go
report, err := service.Run(ctx, "fake://local", 10*time.Second)
```

Assert:

```text
schema_version = dbprobe.inspect/v1alpha1
target.engine = fake
target.adapter_id = fake
capabilities = [activity.sessions, workload.query_summary]
observations contain connections.used=12 and connections.limit=100
deltas contain core.query.calls delta=40 rate=4
findings is an empty JSON array, not null
security.read_only_guaranteed = true
```

Run:

```bash
go test ./internal/app/inspect -v
```

Expected: FAIL.

- [ ] **Step 2: Implement report and service**

Create:

```go
const SchemaVersion = "dbprobe.inspect/v1alpha1"

type Report struct {
    SchemaVersion string                  `json:"schema_version"`
    CollectedAt   time.Time               `json:"collected_at"`
    Target        adapter.TargetMetadata  `json:"target"`
    Capabilities  []capability.Capability `json:"capabilities"`
    Security      adapter.SecurityProfile `json:"security"`
    Observations  []signal.Observation    `json:"observations"`
    Deltas        []signal.Delta          `json:"deltas"`
    Findings      []finding.Finding       `json:"findings"`
    Warnings      []collection.Warning    `json:"warnings"`
}
```

`Service.Run` sequence must be exactly:

```text
parse target
→ resolve adapter
→ open runtime
→ defer close
→ capture capabilities
→ run collection planner
→ evaluate runtime rules whose capabilities are satisfied
→ build stable report
```

For this plan, fake adapter has zero rules so `Findings` is initialized with `make([]finding.Finding, 0)`.

Errors returned to callers must not include the full raw connection URL. Parse/resolve/open errors may include only scheme/adapter-safe context.

- [ ] **Step 3: Run use-case and whole-suite tests**

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
- Consumes: `inspect.Report` only.
- Produces: `jsonsurface.Render(io.Writer, inspect.Report) error`; `terminal.Render(io.Writer, inspect.Report) error`.

- [ ] **Step 1: Write failing JSON contract test**

Construct a fixed report with a fixed timestamp and render it. Unmarshal into `map[string]any` and assert:

- `schema_version` exists and equals `dbprobe.inspect/v1alpha1`;
- `target`, `capabilities`, `security`, `observations`, `deltas`, `findings`, `warnings` are present;
- no Go implementation field names leak into JSON;
- findings serializes as `[]` when empty.

Run:

```bash
go test ./internal/surfaces/json -v
```

Expected: FAIL.

- [ ] **Step 2: Implement JSON renderer**

Use only `encoding/json`:

```go
func Render(w io.Writer, report inspect.Report) error {
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    return enc.Encode(report)
}
```

Do not add an alternate JSON DTO in this plan; `inspect.Report` is the v1alpha1 application contract.

- [ ] **Step 3: Write and implement minimal terminal renderer**

Terminal output for fake adapter must contain these stable facts without ANSI dependence:

```text
dbprobe · fake · local
read-only: yes
capabilities: 2
observations: 3
deltas: 1
findings: 0
```

It may additionally print warnings. Keep this renderer intentionally minimal; dashboard styling belongs to a later plan.

- [ ] **Step 4: Run surface tests**

```bash
go test ./internal/surfaces/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/surfaces
git commit -m "feat: add JSON and terminal report surfaces"
```

---

### Task 8: CLI composition root and first executable vertical slice

**Files:**
- Create: `cmd/dbprobe/main.go`
- Create: `cmd/dbprobe/root.go`
- Create: `cmd/dbprobe/inspect.go`

**Interfaces:**
- Consumes: fake adapter, registry, collection planner, inspect service, surfaces.
- Produces: `dbprobe inspect <target>` with `--format=text|json` and `--sample-window`.

- [ ] **Step 1: Implement root command without business logic**

`root.go` exposes:

```go
func newRootCommand() *cobra.Command
```

with command name `dbprobe`, silence usage on runtime errors, and add the inspect command.

`main.go` only executes the root command and exits non-zero on error.

- [ ] **Step 2: Implement inspect command composition**

`inspect.go` must:

1. require exactly one target argument;
2. accept `--format` default `text`, allowed `text|json`;
3. accept `--sample-window` default `1s`;
4. instantiate registry with `fake.New()`;
5. instantiate planner with `collection.RealWaiter{}` and `time.Now`;
6. create inspect service;
7. render selected surface;
8. reject unsupported formats before opening an adapter.

No concrete adapter import may appear under `internal/surfaces` or `internal/core`; the concrete `fake` import is allowed only in this composition-root phase.

- [ ] **Step 3: Build and run JSON smoke test**

```bash
go build -o ./bin/dbprobe ./cmd/dbprobe
./bin/dbprobe inspect fake://local --format=json --sample-window=10ms
```

Expected JSON includes:

```json
{
  "schema_version": "dbprobe.inspect/v1alpha1",
  "target": {
    "engine": "fake",
    "adapter_id": "fake"
  }
}
```

and one delta where `key` is `core.query.calls`, `delta` is `40`, and `rate_per_second` is approximately `4000` because the smoke window is `10ms`.

- [ ] **Step 4: Run text smoke test**

```bash
./bin/dbprobe inspect fake://local --sample-window=10ms
```

Expected to include:

```text
dbprobe · fake · local
read-only: yes
findings: 0
```

- [ ] **Step 5: Verify invalid target/format behavior**

```bash
./bin/dbprobe inspect redis://local --sample-window=10ms
./bin/dbprobe inspect fake://local --format=xml --sample-window=10ms
```

Expected: both exit non-zero; neither output contains credentials or a Go stack trace.

- [ ] **Step 6: Commit**

```bash
git add cmd/dbprobe
git commit -m "feat: expose inspect vertical slice through CLI"
```

---

### Task 9: Final verification and foundation acceptance gate

**Files:**
- Modify only if verification exposes defects in files created by Tasks 1-8.

**Interfaces:**
- Consumes: entire foundation runtime.
- Produces: evidence that the repository is safe to begin the MySQL adapter plan.

- [ ] **Step 1: Format and static analysis**

```bash
make fmt
make fmt-check
make vet
```

Expected: all exit `0`.

- [ ] **Step 2: Run full tests with race detector**

```bash
go test -race ./...
```

Expected: PASS with no race reports.

- [ ] **Step 3: Run dependency-boundary test explicitly**

```bash
go test ./test/architecture -run TestDependencyBoundaries -v
```

Expected: PASS.

- [ ] **Step 4: Run the executable acceptance examples**

```bash
go build -o ./bin/dbprobe ./cmd/dbprobe
./bin/dbprobe inspect fake://local --format=json --sample-window=10ms > /tmp/dbprobe-report.json
./bin/dbprobe inspect fake://local --sample-window=10ms
```

Verify `/tmp/dbprobe-report.json` parses as JSON and has schema version `dbprobe.inspect/v1alpha1`.

- [ ] **Step 5: Confirm MVP-foundation exit criteria manually**

The plan is complete only when every statement below is true:

```text
[ ] core contains no database driver
[ ] fake adapter resolves from scheme through registry
[ ] capabilities suppress unsupported collectors
[ ] one collector failure does not discard successful evidence
[ ] counter sampling is scheduled centrally by core
[ ] reset counters never produce negative rates
[ ] report contains explicit read-only security profile
[ ] versioned JSON is emitted from the same application report used by terminal rendering
[ ] adapter contract test harness is reusable for MySQL
[ ] architecture import guard runs in CI
[ ] go test -race ./... passes
```

- [ ] **Step 6: Commit verification-only fixes, if any**

If verification required corrections, stage only those corrected files and use:

```bash
git commit -m "fix: satisfy foundation runtime acceptance gate"
```

If no corrections were necessary, do not create an empty commit.

---

## Definition of Done

This plan is done when a fresh checkout can run:

```bash
make ci
go test -race ./...
go build -o ./bin/dbprobe ./cmd/dbprobe
./bin/dbprobe inspect fake://local --format=json --sample-window=10ms
```

and produce a valid `dbprobe.inspect/v1alpha1` report containing deterministic fake observations plus a centrally calculated counter delta.

At that point the repository is ready for the next implementation plan: **MySQL Adapter MVP**, which will add MySQL 8.0/8.4 connectivity, capability/privilege discovery, Performance Schema/sys collectors, and the first engine-native findings without changing the established core dependency direction.
