package terminal_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/app/inspect"
	"github.com/kefyusuf/dbprobe/internal/core/collection"
	"github.com/kefyusuf/dbprobe/internal/surfaces/terminal"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestRenderShowsCompactFakeSummary(t *testing.T) {
	report := inspect.Report{SchemaVersion: inspect.SchemaVersion, CollectedAt: time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC), Target: adapter.TargetMetadata{Engine: "fake", AdapterID: "fake", Fingerprint: "abc", DisplayName: "local"}, Capabilities: []capability.Capability{"activity.sessions", "workload.query_summary"}, Security: adapter.SecurityProfile{ReadOnlyGuaranteed: true}, Observations: []signal.Observation{signal.NumberObservation("a", object.Ref{Kind: "fake.instance", ID: "local"}, 1, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, time.Now()), signal.NumberObservation("b", object.Ref{Kind: "fake.instance", ID: "local"}, 2, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, time.Now()), signal.NumberObservation("c", object.Ref{Kind: "fake.instance", ID: "local"}, 3, signal.UnitCount, signal.ExactnessScraped, signal.SensitivityMetadata, time.Now())}, Deltas: []signal.Delta{{Key: "c"}}, Findings: []finding.Finding{}, Warnings: []collection.Warning{}}
	var buf bytes.Buffer
	if err := terminal.Render(&buf, report); err != nil { t.Fatal(err) }
	wantLines := []string{"dbprobe · fake · local", "read-only: yes", "capabilities: 2", "observations: 3", "deltas: 1", "findings: 0"}
	for _, line := range wantLines { if !strings.Contains(buf.String(), line+"\n") { t.Fatalf("output missing %q:\n%s", line, buf.String()) } }
}
