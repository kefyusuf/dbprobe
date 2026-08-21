package terminal_test

import (
	"bytes"
	"strings"
	"testing"

	appexplain "github.com/kefyusuf/dbprobe/internal/app/explain"
	"github.com/kefyusuf/dbprobe/internal/surfaces/terminal"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

func TestRenderExplainShowsTargetFormatSanitizationAndPlan(t *testing.T) {
	var output bytes.Buffer
	report := appexplain.Report{
		SchemaVersion: appexplain.SchemaVersion,
		Target:        adapter.TargetMetadata{Engine: "mysql", DisplayName: "db/shop"},
		Format:        "mysql-json-sanitized",
		Estimated:     true,
		Sanitized:     true,
		Plan:          `{"query_block":{}}`,
	}
	if err := terminal.RenderExplain(&output, report); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, part := range []string{"mysql", "db/shop", "mysql-json-sanitized", "estimated: true", "sanitized: true", "query_block"} {
		if !strings.Contains(text, part) {
			t.Fatalf("missing %q in %s", part, text)
		}
	}
}
