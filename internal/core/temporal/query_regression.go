package temporal

import (
	"sort"

	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type MetricPair struct {
	CallsKey        signal.Key
	TotalLatencyKey signal.Key
}

type QueryRegressionPolicy struct {
	MinimumCurrentCalls       float64
	MinimumPreviousCalls      float64
	MinimumAbsoluteIncreaseMS float64
	MinimumRatio              float64
}

func DefaultQueryRegressionPolicy() QueryRegressionPolicy {
	return QueryRegressionPolicy{
		MinimumCurrentCalls:       10,
		MinimumPreviousCalls:      5,
		MinimumAbsoluteIncreaseMS: 5,
		MinimumRatio:              2,
	}
}

type QueryRegression struct {
	Object                object.Ref `json:"object"`
	PreviousMeanLatencyMS float64    `json:"previous_mean_latency_ms"`
	CurrentMeanLatencyMS  float64    `json:"current_mean_latency_ms"`
	Ratio                 float64    `json:"ratio"`
	AddedLatencyMS        float64    `json:"added_latency_ms"`
	CurrentCalls          float64    `json:"current_calls"`
}

func DetectQueryRegressions(previous, current Snapshot, metrics MetricPair, policy QueryRegressionPolicy) []QueryRegression {
	if previous.TargetFingerprint == "" || previous.TargetFingerprint != current.TargetFingerprint || metrics.CallsKey == "" || metrics.TotalLatencyKey == "" {
		return nil
	}
	previousMetrics := sampledMetrics(previous.Deltas, metrics)
	currentMetrics := sampledMetrics(current.Deltas, metrics)
	out := make([]QueryRegression, 0)
	for id, currentWindow := range currentMetrics {
		previousWindow, ok := previousMetrics[id]
		if !ok {
			continue
		}
		if currentWindow.calls < policy.MinimumCurrentCalls || previousWindow.calls < policy.MinimumPreviousCalls || currentWindow.calls <= 0 || previousWindow.calls <= 0 {
			continue
		}
		previousMean := previousWindow.latency / previousWindow.calls
		currentMean := currentWindow.latency / currentWindow.calls
		increase := currentMean - previousMean
		if previousMean <= 0 || increase < policy.MinimumAbsoluteIncreaseMS {
			continue
		}
		ratio := currentMean / previousMean
		if ratio < policy.MinimumRatio {
			continue
		}
		out = append(out, QueryRegression{
			Object:                currentWindow.ref,
			PreviousMeanLatencyMS: previousMean,
			CurrentMeanLatencyMS:  currentMean,
			Ratio:                 ratio,
			AddedLatencyMS:        currentWindow.calls * increase,
			CurrentCalls:          currentWindow.calls,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AddedLatencyMS == out[j].AddedLatencyMS {
			return objectIdentity(out[i].Object) < objectIdentity(out[j].Object)
		}
		return out[i].AddedLatencyMS > out[j].AddedLatencyMS
	})
	return out
}

type metricWindow struct {
	ref                   object.Ref
	calls, latency        float64
	hasCalls, hasLatency  bool
}

func sampledMetrics(deltas []signal.Delta, pair MetricPair) map[string]metricWindow {
	out := map[string]metricWindow{}
	for _, delta := range deltas {
		if delta.Exactness != signal.ExactnessSampled || (delta.Key != pair.CallsKey && delta.Key != pair.TotalLatencyKey) || delta.Delta < 0 {
			continue
		}
		id := objectIdentity(delta.Object)
		window := out[id]
		window.ref = delta.Object
		if delta.Key == pair.CallsKey {
			window.calls = delta.Delta
			window.hasCalls = true
		} else {
			window.latency = delta.Delta
			window.hasLatency = true
		}
		out[id] = window
	}
	for id, window := range out {
		if !window.hasCalls || !window.hasLatency {
			delete(out, id)
		}
	}
	return out
}

func objectIdentity(ref object.Ref) string {
	return ref.Kind + "|" + ref.ID
}
