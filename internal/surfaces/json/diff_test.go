package jsonsurface

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	appdiff "github.com/kefyusuf/dbprobe/internal/app/diff"
)

func TestRenderDiffPreservesVersionedContract(t *testing.T) {
	report := appdiff.Report{
		SchemaVersion:       appdiff.SchemaVersion,
		TargetFingerprint:   "target",
		PreviousSnapshotID:  "prev",
		CurrentSnapshotID:   "cur",
		PreviousCollectedAt: time.Unix(1, 0).UTC(),
		CurrentCollectedAt:  time.Unix(2, 0).UTC(),
	}
	var output bytes.Buffer
	if err := RenderDiff(&output, report); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["schema_version"] != appdiff.SchemaVersion || decoded["target_fingerprint"] != "target" {
		t.Fatalf("decoded=%#v", decoded)
	}
	for _, key := range []string{"changes", "query_regressions", "events"} {
		if _, ok := decoded[key].([]any); !ok {
			t.Fatalf("%s is not JSON array: %#v", key, decoded[key])
		}
	}
}
