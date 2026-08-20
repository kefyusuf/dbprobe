package terminal

import (
	"fmt"
	"io"

	"github.com/kefyusuf/dbprobe/internal/app/inspect"
)

func Render(w io.Writer, report inspect.Report) error {
	readOnly := "no"
	if report.Security.ReadOnlyGuaranteed { readOnly = "yes" }
	if _, err := fmt.Fprintf(w, "dbprobe · %s · %s\n", report.Target.Engine, report.Target.DisplayName); err != nil { return err }
	if _, err := fmt.Fprintf(w, "read-only: %s\n", readOnly); err != nil { return err }
	if _, err := fmt.Fprintf(w, "capabilities: %d\n", len(report.Capabilities)); err != nil { return err }
	if _, err := fmt.Fprintf(w, "observations: %d\n", len(report.Observations)); err != nil { return err }
	if _, err := fmt.Fprintf(w, "deltas: %d\n", len(report.Deltas)); err != nil { return err }
	_, err := fmt.Fprintf(w, "findings: %d\n", len(report.Findings))
	return err
}
