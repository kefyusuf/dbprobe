package collectors

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/collector"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

type fingerprintRows struct {
	rows [][]string
	pos  int
}

func (r *fingerprintRows) Next() bool { return r.pos < len(r.rows) }
func (r *fingerprintRows) Scan(dest ...any) error {
	if r.pos >= len(r.rows) || len(dest) != len(r.rows[r.pos]) {
		return fmt.Errorf("invalid fingerprint fixture scan")
	}
	for i := range dest {
		value, ok := dest[i].(*string)
		if !ok {
			return fmt.Errorf("unexpected fingerprint scan destination %d", i)
		}
		*value = r.rows[r.pos][i]
	}
	r.pos++
	return nil
}
func (*fingerprintRows) Close() error { return nil }
func (*fingerprintRows) Err() error   { return nil }

type fingerprintQueryer struct {
	groups map[string][][]string
	fail   string
	calls  []string
	args   [][]any
}

func (q *fingerprintQueryer) QueryContext(_ context.Context, query string, args ...any) (Rows, error) {
	q.calls = append(q.calls, query)
	q.args = append(q.args, append([]any(nil), args...))
	group := ""
	switch {
	case strings.Contains(query, "information_schema.check_constraints"):
		group = "checks"
	case strings.Contains(query, "information_schema.referential_constraints"):
		group = "referential"
	case strings.Contains(query, "information_schema.table_constraints"):
		group = "constraints"
	case strings.Contains(query, "information_schema.tables"):
		group = "tables"
	case strings.Contains(query, "information_schema.columns"):
		group = "columns"
	case strings.Contains(query, "information_schema.statistics"):
		group = "statistics"
	}
	if group == q.fail {
		return nil, errors.New("metadata unavailable")
	}
	return &fingerprintRows{rows: q.groups[group]}, nil
}

func fingerprintFixtureGroups() map[string][][]string {
	return map[string][][]string{
		"tables": {{"shop", "orders", "BASE TABLE", "InnoDB", "Dynamic", "utf8mb4_0900_ai_ci"}},
		"columns": {
			{"shop", "orders", "1", "id", "bigint unsigned", "NO", "1", "", "auto_increment", "", ""},
			{"shop", "orders", "2", "code", "varchar(32)", "NO", "0", "", "", "utf8mb4_0900_ai_ci", ""},
		},
		"statistics": {
			{"shop", "orders", "PRIMARY", "0", "1", "id", "", "", "A", "BTREE", "YES"},
			{"shop", "orders", "idx_code", "1", "1", "code", "", "8", "A", "BTREE", "YES"},
		},
		"constraints": {
			{"shop", "orders", "PRIMARY", "PRIMARY KEY", "1", "id", "", "", ""},
			{"shop", "orders", "chk_total", "CHECK", "", "", "", "", ""},
			{"shop", "orders", "fk_customer", "FOREIGN KEY", "1", "customer_id", "shop", "customers", "id"},
		},
		"checks": {
			{"shop", "orders", "chk_total", "(`total_cents` > 0)"},
		},
		"referential": {
			{"shop", "orders", "fk_customer", "shop", "PRIMARY", "NONE", "RESTRICT", "CASCADE"},
		},
	}
}

func cloneFingerprintGroups(in map[string][][]string) map[string][][]string {
	out := map[string][][]string{}
	for key, rows := range in {
		for _, row := range rows {
			out[key] = append(out[key], append([]string(nil), row...))
		}
	}
	return out
}

func collectFingerprintValue(t *testing.T, query Queryer) string {
	t.Helper()
	got, err := NewSchemaFingerprint(query, "shop").Collect(context.Background(), collector.Request{CollectedAt: time.Unix(1, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Text == nil {
		t.Fatalf("observations=%#v", got)
	}
	return *got[0].Text
}

func TestSchemaFingerprintStableAcrossRowOrder(t *testing.T) {
	first := fingerprintFixtureGroups()
	second := cloneFingerprintGroups(first)
	second["columns"][0], second["columns"][1] = second["columns"][1], second["columns"][0]
	second["statistics"][0], second["statistics"][1] = second["statistics"][1], second["statistics"][0]
	second["constraints"][0], second["constraints"][2] = second["constraints"][2], second["constraints"][0]

	firstFingerprint := collectFingerprintValue(t, &fingerprintQueryer{groups: first})
	secondFingerprint := collectFingerprintValue(t, &fingerprintQueryer{groups: second})
	if firstFingerprint != secondFingerprint {
		t.Fatalf("row order changed fingerprint: %s vs %s", firstFingerprint, secondFingerprint)
	}
	if !strings.HasPrefix(firstFingerprint, "v1:sha256:") || len(firstFingerprint) != len("v1:sha256:")+64 {
		t.Fatalf("fingerprint=%q", firstFingerprint)
	}
}

func TestSchemaFingerprintChangesForStructuralMetadata(t *testing.T) {
	base := fingerprintFixtureGroups()
	original := collectFingerprintValue(t, &fingerprintQueryer{groups: base})
	mutations := []func(map[string][][]string){
		func(groups map[string][][]string) { groups["columns"][1][4] = "varchar(64)" },
		func(groups map[string][][]string) { groups["columns"][1][6], groups["columns"][1][7] = "0", "hello" },
		func(groups map[string][][]string) { groups["statistics"][1][7] = "4" },
		func(groups map[string][][]string) { groups["statistics"][1][8] = "D" },
		func(groups map[string][][]string) { groups["constraints"][2][7] = "accounts" },
		func(groups map[string][][]string) { groups["checks"][0][3] = "(`total_cents` >= 0)" },
		func(groups map[string][][]string) { groups["referential"][0][6] = "CASCADE" },
		func(groups map[string][][]string) { groups["referential"][0][7] = "SET NULL" },
	}
	for i, mutate := range mutations {
		groups := cloneFingerprintGroups(base)
		mutate(groups)
		if got := collectFingerprintValue(t, &fingerprintQueryer{groups: groups}); got == original {
			t.Fatalf("mutation %d did not change fingerprint", i)
		}
	}
}

func TestSchemaFingerprintLengthPrefixAvoidsDelimiterCollision(t *testing.T) {
	first := fingerprintFixtureGroups()
	second := fingerprintFixtureGroups()
	first["tables"][0][1], first["tables"][0][2] = "a|b", "c"
	second["tables"][0][1], second["tables"][0][2] = "a", "b|c"
	if collectFingerprintValue(t, &fingerprintQueryer{groups: first}) == collectFingerprintValue(t, &fingerprintQueryer{groups: second}) {
		t.Fatal("length-prefixed canonicalization collided")
	}
}

func TestSchemaFingerprintFailsClosedOnPartialMetadata(t *testing.T) {
	for _, group := range []string{"tables", "columns", "statistics", "constraints", "checks", "referential"} {
		query := &fingerprintQueryer{groups: fingerprintFixtureGroups(), fail: group}
		got, err := NewSchemaFingerprint(query, "shop").Collect(context.Background(), collector.Request{})
		if err == nil || len(got) != 0 {
			t.Fatalf("group=%s observations=%#v error=%v", group, got, err)
		}
	}
}

func TestSchemaFingerprintRejectsOversizedMetadataField(t *testing.T) {
	groups := fingerprintFixtureGroups()
	groups["columns"][0][4] = strings.Repeat("x", maxSchemaFingerprintFieldBytes+1)
	got, err := NewSchemaFingerprint(&fingerprintQueryer{groups: groups}, "shop").Collect(context.Background(), collector.Request{})
	if err == nil || len(got) != 0 {
		t.Fatalf("observations=%#v error=%v", got, err)
	}
}

func TestSchemaFingerprintScopesQueriesAndEmitsOnlyOpaqueDigest(t *testing.T) {
	query := &fingerprintQueryer{groups: fingerprintFixtureGroups()}
	got, err := NewSchemaFingerprint(query, "shop").Collect(context.Background(), collector.Request{CollectedAt: time.Unix(2, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if len(query.calls) != 6 {
		t.Fatalf("query calls=%d", len(query.calls))
	}
	for i, args := range query.args {
		if len(args) != 1 || args[0] != "shop" {
			t.Fatalf("args[%d]=%#v", i, args)
		}
		if !strings.Contains(query.calls[i], "ORDER BY") {
			t.Fatalf("query lacks deterministic ordering: %s", query.calls[i])
		}
	}
	if len(got) != 1 || got[0].Key != "mysql.schema.structural_fingerprint" || got[0].Object.Kind != "mysql.schema" || got[0].Object.ID != "shop" || got[0].Exactness != signal.ExactnessScraped || got[0].Sensitivity != signal.SensitivityMetadata || got[0].Text == nil {
		t.Fatalf("observation=%#v", got)
	}
	for _, rawMetadata := range []string{"orders", "varchar(32)", "idx_code", "total_cents", "CASCADE"} {
		if strings.Contains(*got[0].Text, rawMetadata) {
			t.Fatalf("fingerprint leaks raw metadata %q", rawMetadata)
		}
	}
}
