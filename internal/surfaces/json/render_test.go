package jsonsurface_test

import (
	"bytes"
	stdjson "encoding/json"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/app/inspect"
	"github.com/kefyusuf/dbprobe/internal/core/collection"
	jsonsurface "github.com/kefyusuf/dbprobe/internal/surfaces/json"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestRenderPreservesVersionedTopLevelJSONContract(t *testing.T) {
	report := inspect.Report{SchemaVersion: inspect.SchemaVersion, CollectedAt: time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC), Target: adapter.TargetMetadata{Engine: "fake", AdapterID: "fake", Fingerprint: "abc", DisplayName: "local"}, Capabilities: []capability.Capability{}, Security: adapter.SecurityProfile{ReadOnlyGuaranteed: true}, Observations: []signal.Observation{}, Deltas: []signal.Delta{}, Findings: []finding.Finding{}, Warnings: []collection.Warning{}}
	var buf bytes.Buffer
	if err := jsonsurface.Render(&buf, report); err != nil { t.Fatal(err) }
	var decoded map[string]any
	if err := stdjson.Unmarshal(buf.Bytes(), &decoded); err != nil { t.Fatal(err) }
	gotKeys := make([]string, 0, len(decoded)); for key := range decoded { gotKeys = append(gotKeys, key) }; sort.Strings(gotKeys)
	wantKeys := []string{"capabilities", "collected_at", "deltas", "findings", "observations", "schema_version", "security", "target", "warnings"}
	if !reflect.DeepEqual(gotKeys, wantKeys) { t.Fatalf("top-level keys = %#v; want %#v", gotKeys, wantKeys) }
	if decoded["schema_version"] != inspect.SchemaVersion { t.Fatalf("schema_version = %#v", decoded["schema_version"]) }
	findings, ok := decoded["findings"].([]any); if !ok || findings == nil || len(findings) != 0 { t.Fatalf("findings = %#v; want []", decoded["findings"]) }
}
