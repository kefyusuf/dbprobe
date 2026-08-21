package collectors

import (
	"context"
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/collector"
)

func TestPrimaryKeyCollectorEmitsStableTablePresence(t *testing.T) {
	q := &recordingQueryer{rows: [][]string{{"shop", "orders", "1"}, {"shop", "audit_log", "0"}}}
	collectors := NewSchemaRisk(q, "shop", 100)
	if len(collectors) != 2 {
		t.Fatalf("collectors = %d; want 2", len(collectors))
	}
	got, err := collectors[0].Collect(context.Background(), collector.Request{})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, observation := range got {
		if observation.Key != "mysql.table.primary_key_present" || observation.Boolean == nil {
			continue
		}
		seen[observation.Object.ID] = *observation.Boolean
	}
	if !seen["shop.orders"] || seen["shop.audit_log"] {
		t.Fatalf("primary-key observations = %#v", seen)
	}
}

func TestAutoIncrementCollectorComputesTypeRangeUtilization(t *testing.T) {
	q := &recordingQueryer{rows: [][]string{{"shop", "orders", "id", "int", "int unsigned", "4080218931"}}}
	collectors := NewSchemaRisk(q, "shop", 100)
	got, err := collectors[1].Collect(context.Background(), collector.Request{})
	if err != nil {
		t.Fatal(err)
	}
	var ratio float64
	for _, observation := range got {
		if observation.Key == "mysql.auto_increment.utilization_ratio" && observation.Number != nil {
			ratio = *observation.Number
			if observation.Object.Kind != "mysql.column" || observation.Object.ID != "shop.orders.id" {
				t.Fatalf("object = %#v", observation.Object)
			}
		}
	}
	if ratio < 0.94 || ratio > 0.96 {
		t.Fatalf("ratio = %v; want about .95", ratio)
	}
}

func TestAutoIncrementRangeSupportsSignedAndUnsignedIntegerFamilies(t *testing.T) {
	cases := []struct {
		dataType   string
		columnType string
		want       float64
	}{
		{"tinyint", "tinyint", 127},
		{"tinyint", "tinyint unsigned", 255},
		{"smallint", "smallint", 32767},
		{"mediumint", "mediumint unsigned", 16777215},
		{"int", "int unsigned", 4294967295},
		{"bigint", "bigint", 9223372036854775807},
	}
	for _, tc := range cases {
		got, ok := autoIncrementMax(tc.dataType, tc.columnType)
		if !ok || got != tc.want {
			t.Fatalf("%s %s max=%v ok=%v want=%v", tc.dataType, tc.columnType, got, ok, tc.want)
		}
	}
}
