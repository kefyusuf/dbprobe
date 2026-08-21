package terminal

import (
	"bytes"
	"strings"
	"testing"
	"time"

	appdiff "github.com/kefyusuf/dbprobe/internal/app/diff"
	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/sdk/object"
)

func TestRenderDiffSummarizesTemporalReport(t *testing.T) {
	report := appdiff.Report{
		SchemaVersion:       appdiff.SchemaVersion,
		TargetFingerprint:   "target",
		PreviousSnapshotID:  "prev",
		CurrentSnapshotID:   "cur",
		PreviousCollectedAt: time.Unix(1, 0).UTC(),
		CurrentCollectedAt:  time.Unix(2, 0).UTC(),
		Changes:             []temporal.Change{{Kind: temporal.ChangeReset}},
		QueryRegressions:    []temporal.QueryRegression{{Object: object.Ref{Kind: "mysql.query", ID: "q"}}},
		Events: []temporal.Event{{
			Type:    temporal.EventQueryRegression,
			Object:  object.Ref{Kind: "mysql.query", ID: "q"},
			Summary: "latency regressed",
		}},
	}
	var output bytes.Buffer
	if err := RenderDiff(&output, report); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, want := range []string{"dbprobe diff · target", "changes: 1", "query regressions: 1", "events: 1", "query_regression · mysql.query:q · latency regressed"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
}
