package collection

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

const defaultMaxConcurrency = 4

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

type RealWaiter struct{}

func (RealWaiter) Wait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type Planner struct {
	waiter         Waiter
	now            func() time.Time
	maxConcurrency int
}

func New(waiter Waiter, now func() time.Time) *Planner {
	return &Planner{waiter: waiter, now: now, maxConcurrency: defaultMaxConcurrency}
}

func (p *Planner) WithMaxConcurrency(value int) *Planner {
	if value < 1 {
		value = 1
	}
	p.maxConcurrency = value
	return p
}

type batchJob struct {
	index      int
	collector  collector.Collector
	descriptor collector.Descriptor
	request    collector.Request
}

type batchResult struct {
	index      int
	descriptor collector.Descriptor
	observed   []signal.Observation
	err        error
}

func (p *Planner) Run(ctx context.Context, caps capability.Set, collectors []collector.Collector, sampleWindow time.Duration) (Result, error) {
	result := Result{
		Observations: []signal.Observation{},
		Deltas:       []signal.Delta{},
		Warnings:     []Warning{},
	}

	snapshots := make([]collector.Collector, 0, len(collectors))
	counters := make([]collector.Collector, 0, len(collectors))
	for _, candidate := range collectors {
		descriptor := candidate.Descriptor()
		if !caps.HasAll(descriptor.Requires) {
			continue
		}
		switch descriptor.Strategy {
		case collector.StrategySnapshot:
			snapshots = append(snapshots, candidate)
		case collector.StrategyCounter:
			counters = append(counters, candidate)
		default:
			result.Warnings = append(result.Warnings, Warning{CollectorID: descriptor.ID, Reason: "unsupported collection strategy"})
		}
	}

	snapshotResults, err := p.runBatch(ctx, p.jobs(snapshots, collector.PhaseSingle))
	if err != nil {
		return Result{}, err
	}
	for _, collected := range snapshotResults {
		result.Observations = append(result.Observations, collected.observed...)
		if collected.err != nil {
			result.Warnings = append(result.Warnings, Warning{CollectorID: collected.descriptor.ID, Reason: collected.err.Error()})
		}
	}

	firstSamples := make(map[string]signal.Observation)
	if len(counters) > 0 {
		if sampleWindow <= 0 {
			return Result{}, fmt.Errorf("sample window must be positive for counter collectors")
		}

		firstResults, err := p.runBatch(ctx, p.jobs(counters, collector.PhaseSampleA))
		if err != nil {
			return Result{}, err
		}
		for _, collected := range firstResults {
			if collected.err != nil {
				result.Warnings = append(result.Warnings, Warning{CollectorID: collected.descriptor.ID, Reason: collected.err.Error()})
			}
			for _, observation := range collected.observed {
				firstSamples[collected.descriptor.ID+"|"+identity(observation)] = observation
			}
		}

		if err := p.waiter.Wait(ctx, sampleWindow); err != nil {
			return Result{}, err
		}

		secondResults, err := p.runBatch(ctx, p.jobs(counters, collector.PhaseSampleB))
		if err != nil {
			return Result{}, err
		}
		for _, collected := range secondResults {
			result.Observations = append(result.Observations, collected.observed...)
			if collected.err != nil {
				result.Warnings = append(result.Warnings, Warning{CollectorID: collected.descriptor.ID, Reason: collected.err.Error()})
			}
			for _, current := range collected.observed {
				previous, ok := firstSamples[collected.descriptor.ID+"|"+identity(current)]
				if !ok {
					continue
				}
				a, aOK := previous.Numeric()
				b, bOK := current.Numeric()
				if !aOK || !bOK {
					continue
				}
				if b < a {
					result.Warnings = append(result.Warnings, Warning{CollectorID: collected.descriptor.ID, Reason: fmt.Sprintf("counter reset detected for %s", identity(current))})
					continue
				}
				delta := b - a
				result.Deltas = append(result.Deltas, signal.Delta{
					Key:           current.Key,
					Object:        current.Object,
					Unit:          current.Unit,
					Delta:         delta,
					RatePerSecond: delta / sampleWindow.Seconds(),
					WindowSeconds: sampleWindow.Seconds(),
					Exactness:     signal.ExactnessSampled,
				})
			}
		}
	}

	sortResult(&result)
	return result, nil
}

func (p *Planner) jobs(values []collector.Collector, phase collector.Phase) []batchJob {
	jobs := make([]batchJob, 0, len(values))
	for index, candidate := range values {
		jobs = append(jobs, batchJob{
			index:      index,
			collector:  candidate,
			descriptor: candidate.Descriptor(),
			request:    collector.Request{Phase: phase, CollectedAt: p.now()},
		})
	}
	return jobs
}

func (p *Planner) runBatch(ctx context.Context, jobs []batchJob) ([]batchResult, error) {
	if len(jobs) == 0 {
		return []batchResult{}, nil
	}
	limit := p.maxConcurrency
	if limit < 1 {
		limit = 1
	}
	if limit > len(jobs) {
		limit = len(jobs)
	}

	jobCh := make(chan batchJob)
	resultCh := make(chan batchResult, len(jobs))
	var workers sync.WaitGroup
	workers.Add(limit)
	for i := 0; i < limit; i++ {
		go func() {
			defer workers.Done()
			for job := range jobCh {
				if ctx.Err() != nil {
					return
				}
				observed, err := job.collector.Collect(ctx, job.request)
				resultCh <- batchResult{index: job.index, descriptor: job.descriptor, observed: observed, err: err}
			}
		}()
	}

	go func() {
		defer close(jobCh)
		for _, job := range jobs {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case jobCh <- job:
			}
		}
	}()

	workers.Wait()
	close(resultCh)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	results := make([]batchResult, 0, len(jobs))
	for collected := range resultCh {
		results = append(results, collected)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].index < results[j].index })
	return results, nil
}

func sortResult(result *Result) {
	sort.Slice(result.Observations, func(i, j int) bool { return identity(result.Observations[i]) < identity(result.Observations[j]) })
	sort.Slice(result.Deltas, func(i, j int) bool {
		left := fmt.Sprintf("%s|%s|%s", result.Deltas[i].Key, result.Deltas[i].Object.Kind, result.Deltas[i].Object.ID)
		right := fmt.Sprintf("%s|%s|%s", result.Deltas[j].Key, result.Deltas[j].Object.Kind, result.Deltas[j].Object.ID)
		return left < right
	})
	sort.Slice(result.Warnings, func(i, j int) bool {
		if result.Warnings[i].CollectorID == result.Warnings[j].CollectorID {
			return result.Warnings[i].Reason < result.Warnings[j].Reason
		}
		return result.Warnings[i].CollectorID < result.Warnings[j].CollectorID
	})
}

func identity(o signal.Observation) string {
	return fmt.Sprintf("%s|%s|%s", o.Key, o.Object.Kind, o.Object.ID)
}
