package terminal

import (
	"fmt"
	"io"

	appdiff "github.com/kefyusuf/dbprobe/internal/app/diff"
)

func RenderDiff(w io.Writer, report appdiff.Report) error {
	if _, err := fmt.Fprintf(w, "dbprobe diff · %s\n", report.TargetFingerprint); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "previous: %s\n", report.PreviousCollectedAt.UTC().Format("2006-01-02T15:04:05Z07:00")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "current: %s\n", report.CurrentCollectedAt.UTC().Format("2006-01-02T15:04:05Z07:00")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "changes: %d\nquery regressions: %d\nevents: %d\n", len(report.Changes), len(report.QueryRegressions), len(report.Events)); err != nil {
		return err
	}
	for _, event := range report.Events {
		if _, err := fmt.Fprintf(w, "%s · %s:%s · %s\n", event.Type, event.Object.Kind, event.Object.ID, event.Summary); err != nil {
			return err
		}
	}
	return nil
}
