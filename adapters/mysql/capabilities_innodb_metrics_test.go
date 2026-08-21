package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/capability"
)

func TestDiscoverCapabilitiesSeparatesInnoDBEngineFromMetricsVisibility(t *testing.T) {
	caps := discoverCapabilities(context.Background(), false, func(_ context.Context, query string) error {
		if query == probeInnoDB {
			return nil
		}
		return errors.New("unavailable")
	})
	if !caps.Has(capability.Capability("mysql.innodb")) {
		t.Fatal("missing mysql.innodb")
	}
	if caps.Has(capability.Capability("mysql.innodb_metrics")) {
		t.Fatal("claimed mysql.innodb_metrics without metrics visibility")
	}

	caps = discoverCapabilities(context.Background(), false, func(_ context.Context, query string) error {
		if query == probeInnoDB || query == probeInnoDBMetrics {
			return nil
		}
		return errors.New("unavailable")
	})
	if !caps.Has(capability.Capability("mysql.innodb_metrics")) {
		t.Fatal("missing mysql.innodb_metrics")
	}
}
