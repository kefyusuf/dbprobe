# Foundation Runtime Plan — Execution Notes

These notes are normative clarifications for `2026-08-21-dbprobe-foundation-runtime.md` discovered during plan self-review.

## Planner signature

The collection planner public method used by Tasks 4-6 is:

```go
func (p *Planner) Run(
    ctx context.Context,
    caps capability.Set,
    collectors []collector.Collector,
    sampleWindow time.Duration,
) Result
```

## Inspect service constructor

The application service used by Task 6 is constructed through:

```go
type Service struct {
    registry *adapterregistry.Registry
    planner  *collection.Planner
    now      func() time.Time
}

func New(
    registry *adapterregistry.Registry,
    planner *collection.Planner,
    now func() time.Time,
) *Service
```

`Service.Run` is:

```go
func (s *Service) Run(
    ctx context.Context,
    rawTarget string,
    sampleWindow time.Duration,
) (Report, error)
```

## Service test waiter

`internal/app/inspect/service_test.go` defines its own local no-wait implementation; it does not rely on a test-only symbol from the collection package:

```go
type noopWaiter struct{}

func (noopWaiter) Wait(context.Context, time.Duration) error { return nil }
```

These signatures override less-specific wording in the main foundation plan. No implementation scope is added by this note.
