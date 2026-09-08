package temporal

import (
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestDetectQueryRegressionsUsesSampledDeltasOnly(t *testing.T) {
	at := time.Now()
	query := object.Ref{Kind: "mysql.query", ID: "shop:abc"}
	metrics := MetricPair{CallsKey: "core.query.calls", TotalLatencyKey: "mysql.query.total_latency_ms"}
	previous := mustSnapshot(t, at, nil, []signal.Delta{
		{Key: "core.query.calls", Object: query, Delta: 20, Exactness: signal.ExactnessSampled, WindowSeconds: 10},
		{Key: "mysql.query.total_latency_ms", Object: query, Delta: 200, Exactness: signal.ExactnessSampled, WindowSeconds: 10},
	})
	current := mustSnapshot(t, at.Add(time.Minute), nil, []signal.Delta{
		{Key: "core.query.calls", Object: query, Delta: 30, Exactness: signal.ExactnessSampled, WindowSeconds: 10},
		{Key: "mysql.query.total_latency_ms", Object: query, Delta: 900, Exactness: signal.ExactnessSampled, WindowSeconds: 10},
	})
	got := DetectQueryRegressions(previous, current, metrics, DefaultQueryRegressionPolicy())
	if len(got) != 1 {
		t.Fatalf("got=%#v", got)
	}
	if got[0].PreviousMeanLatencyMS != 10 || got[0].CurrentMeanLatencyMS != 30 || got[0].Ratio != 3 || got[0].AddedLatencyMS != 600 {
		t.Fatalf("regression=%#v", got[0])
	}
}

func TestDetectQueryRegressionsIgnoresLowVolumeAndCumulativeObservations(t *testing.T) {
	at := time.Now()
	query := object.Ref{Kind: "mysql.query", ID: "q"}
	metrics := MetricPair{CallsKey: "core.query.calls", TotalLatencyKey: "latency"}
	previous := mustSnapshot(t, at,
		[]signal.Observation{signal.NumberObservation("latency", query, 99999, signal.UnitMilliseconds, signal.ExactnessCumulative, signal.SensitivityMetadata, at)},
		[]signal.Delta{{Key: "core.query.calls", Object: query, Delta: 4, Exactness: signal.ExactnessSampled}, {Key: "latency", Object: query, Delta: 20, Exactness: signal.ExactnessSampled}},
	)
	current := mustSnapshot(t, at.Add(time.Minute),
		[]signal.Observation{signal.NumberObservation("latency", query, 999999, signal.UnitMilliseconds, signal.ExactnessCumulative, signal.SensitivityMetadata, at)},
		[]signal.Delta{{Key: "core.query.calls", Object: query, Delta: 9, Exactness: signal.ExactnessSampled}, {Key: "latency", Object: query, Delta: 9000, Exactness: signal.ExactnessSampled}},
	)
	if got := DetectQueryRegressions(previous, current, metrics, DefaultQueryRegressionPolicy()); len(got) != 0 {
		t.Fatalf("unexpected=%#v", got)
	}
}

func mustSnapshot(t *testing.T, at time.Time, observations []signal.Observation, deltas []signal.Delta) Snapshot {
	return mustSnapshotFor(t, "target", at, observations, deltas)
}

func mustSnapshotFor(t *testing.T, target string, at time.Time, observations []signal.Observation, deltas []signal.Delta) Snapshot {
	t.Helper()
	snapshot, err := NewSnapshot(SnapshotInput{
		TargetFingerprint: target,
		Engine:            "test",
		AdapterID:         "test",
		AdapterVersion:    "1",
		CollectedAt:       at,
		Observations:      observations,
		Deltas:            deltas,
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
