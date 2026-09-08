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
