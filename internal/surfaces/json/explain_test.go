package json_test

import (
	"bytes"
	"strings"
	"testing"

	appexplain "github.com/kefyusuf/dbprobe/internal/app/explain"
	jsonsurface "github.com/kefyusuf/dbprobe/internal/surfaces/json"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

func TestRenderExplainUsesVersionedSanitizedStringPlan(t *testing.T) {
	var output bytes.Buffer
	report := appexplain.Report{
		SchemaVersion: appexplain.SchemaVersion,
		Target:        adapter.TargetMetadata{Engine: "mysql"},
		Format:        "mysql-json-sanitized",
		Estimated:     true,
		Sanitized:     true,
		Plan:          `{"query_block":{}}`,
	}
	if err := jsonsurface.RenderExplain(&output, report); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	if !strings.Contains(text, "dbprobe.explain/v1alpha1") || !strings.Contains(text, "mysql-json-sanitized") || !strings.Contains(text, `"sanitized": true`) || strings.Contains(text, "ZXhwbGFpbg==") {
		t.Fatalf("json=%s", text)
	}
}
