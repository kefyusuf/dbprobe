package collectors

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/collector"
)

func TestSchemaFingerprintRejectsExcessiveTotalMetadata(t *testing.T) {
	groups := fingerprintFixtureGroups()
	largeClause := strings.Repeat("x", maxSchemaFingerprintFieldBytes)
	groups["checks"] = make([][]string, 65)
	for i := range groups["checks"] {
		groups["checks"][i] = []string{"shop", "orders", fmt.Sprintf("chk_%03d", i), largeClause}
	}
	got, err := NewSchemaFingerprint(&fingerprintQueryer{groups: groups}, "shop").Collect(context.Background(), collector.Request{})
	if err == nil || len(got) != 0 {
		t.Fatalf("observations=%d error=%v", len(got), err)
	}
}
