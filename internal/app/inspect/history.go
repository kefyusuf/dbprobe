package inspect

import (
	"context"

	"github.com/kefyusuf/dbprobe/internal/core/collection"
	"github.com/kefyusuf/dbprobe/internal/core/temporal"
)

func persistHistory(ctx context.Context, store temporal.Store, report Report, adapterVersion string) *collection.Warning {
	if store == nil {
		return nil
	}
	if len(report.Warnings) > 0 {
		return &collection.Warning{CollectorID: "history", Reason: "snapshot persistence skipped because inspection evidence is incomplete"}
	}
	snapshot, err := temporal.NewSnapshot(temporal.SnapshotInput{
		TargetFingerprint: report.Target.Fingerprint,
		Engine:            report.Target.Engine,
		AdapterID:         report.Target.AdapterID,
		AdapterVersion:    adapterVersion,
		CollectedAt:       report.CollectedAt,
		Capabilities:      report.Capabilities,
		Observations:      report.Observations,
		Deltas:            report.Deltas,
		Findings:          report.Findings,
	})
	if err != nil {
		return &collection.Warning{CollectorID: "history", Reason: "snapshot persistence failed"}
	}
	if err := store.Save(ctx, snapshot); err != nil {
		return &collection.Warning{CollectorID: "history", Reason: "snapshot persistence failed"}
	}
	return nil
}
