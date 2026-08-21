package terminal

import (
	"fmt"
	"io"

	appexplain "github.com/kefyusuf/dbprobe/internal/app/explain"
)

func RenderExplain(w io.Writer, report appexplain.Report) error {
	if _, err := fmt.Fprintf(w, "dbprobe explain · %s · %s\n", report.Target.Engine, report.Target.DisplayName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "format: %s · estimated: %t\n\n", report.Format, report.Estimated); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, report.Plan)
	return err
}
