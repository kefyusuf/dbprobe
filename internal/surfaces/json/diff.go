package jsonsurface

import (
	"encoding/json"
	"io"

	appdiff "github.com/kefyusuf/dbprobe/internal/app/diff"
	"github.com/kefyusuf/dbprobe/internal/core/temporal"
)

func RenderDiff(w io.Writer, report appdiff.Report) error {
	if report.Changes == nil {
		report.Changes = []temporal.Change{}
	}
	if report.QueryRegressions == nil {
		report.QueryRegressions = []temporal.QueryRegression{}
	}
	if report.Events == nil {
		report.Events = []temporal.Event{}
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
