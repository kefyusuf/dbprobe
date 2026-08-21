package mysql

import (
	"strings"
	"testing"
)

func TestSanitizeMySQLJSONPlanAllowlistsScalarMetadata(t *testing.T) {
	raw := `{"query_block":{"select_id":1,"unsafe_numeric":123456,"unsafe_bool":true,"cost_info":{"query_cost":"1.20"},"table":{"table_name":"orders","access_type":"ref","possible_keys":["idx_email"],"key":"idx_email","rows_examined_per_scan":1,"attached_condition":"orders.email = 'secret@example.test'","literal_number":424242,"ref":["const"],"using_index":true}}}`
	got, err := sanitizeMySQLJSONPlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"select_id", "query_cost", "orders", "idx_email", "rows_examined_per_scan", "using_index"} {
		if !strings.Contains(got, required) {
			t.Fatalf("sanitized plan missing %q: %s", required, got)
		}
	}
	for _, forbidden := range []string{"secret@example.test", "attached_condition", "unsafe_numeric", "123456", "unsafe_bool", "literal_number", "424242", `"ref":`} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("sanitized plan leaked %q: %s", forbidden, got)
		}
	}
}

func TestSanitizeMySQLJSONPlanRejectsInvalidEmptyAndOversizedPlans(t *testing.T) {
	cases := []string{
		`[]`,
		`{"attached_condition":"secret"}`,
		`{} {}`,
		strings.Repeat("x", maxExplainPlanBytes+1),
	}
	for _, raw := range cases {
		if _, err := sanitizeMySQLJSONPlan(raw); err == nil {
			t.Fatalf("accepted invalid plan len=%d", len(raw))
		}
	}
}
