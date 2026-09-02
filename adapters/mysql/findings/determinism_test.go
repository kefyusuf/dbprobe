package findings

import (
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/sdk/finding"
	"github.com/kefyusuf/dbprobe/sdk/object"
	"github.com/kefyusuf/dbprobe/sdk/signal"
)

func TestGroupedRulesEmitFindingsInStableObjectOrder(t *testing.T) {
	queryA := object.Ref{Kind: "mysql.query", ID: "query-a"}
	queryB := object.Ref{Kind: "mysql.query", ID: "query-b"}
	instanceA := object.Ref{Kind: "mysql.instance", ID: "instance-a"}
	instanceB := object.Ref{Kind: "mysql.instance", ID: "instance-b"}
	tableA := object.Ref{Kind: "mysql.table", ID: "shop.table-a"}
	tableB := object.Ref{Kind: "mysql.table", ID: "shop.table-b"}

	cases := []struct {
		name string
		rule finding.Rule
		ctx  finding.AnalysisContext
		want [2]string
	}{
		{
			name: "query full scan",
			rule: queryFullScanRule{},
			ctx: finding.AnalysisContext{Deltas: []signal.Delta{
				{Key: "mysql.query.no_index_used", Object: queryB, Delta: 15},
				{Key: "core.query.calls", Object: queryA, Delta: 20},
				{Key: "core.query.calls", Object: queryB, Delta: 20},
				{Key: "mysql.query.no_index_used", Object: queryA, Delta: 15},
			}},
			want: [2]string{"query-a", "query-b"},
		},
		{
			name: "query amplification",
			rule: queryAmplificationRule{},
			ctx: finding.AnalysisContext{Deltas: []signal.Delta{
				{Key: "mysql.query.rows_examined", Object: queryB, Delta: 20000},
				{Key: "mysql.query.rows_sent", Object: queryA, Delta: 10},
				{Key: "mysql.query.rows_sent", Object: queryB, Delta: 10},
				{Key: "mysql.query.rows_examined", Object: queryA, Delta: 20000},
			}},
			want: [2]string{"query-a", "query-b"},
		},
		{
			name: "buffer pool hit",
			rule: bufferPoolHitRule{},
			ctx: finding.AnalysisContext{Deltas: []signal.Delta{
				{Key: "mysql.innodb.buffer_pool.reads", Object: instanceB, Delta: 500},
				{Key: "mysql.innodb.buffer_pool.read_requests", Object: instanceA, Delta: 1000},
				{Key: "mysql.innodb.buffer_pool.read_requests", Object: instanceB, Delta: 1000},
				{Key: "mysql.innodb.buffer_pool.reads", Object: instanceA, Delta: 500},
			}},
			want: [2]string{"instance-a", "instance-b"},
		},
		{
			name: "no good index",
			rule: noGoodIndexRule{},
			ctx: finding.AnalysisContext{Deltas: []signal.Delta{
				{Key: "mysql.query.no_good_index_used", Object: queryB, Delta: 20},
				{Key: "core.query.calls", Object: queryA, Delta: 20},
				{Key: "core.query.calls", Object: queryB, Delta: 20},
				{Key: "mysql.query.no_good_index_used", Object: queryA, Delta: 20},
			}},
			want: [2]string{"query-a", "query-b"},
		},
		{
			name: "disk temp table",
			rule: diskTempTableRule{},
			ctx: finding.AnalysisContext{Deltas: []signal.Delta{
				{Key: "mysql.query.temp_disk_tables", Object: queryB, Delta: 12},
				{Key: "core.query.calls", Object: queryA, Delta: 20},
				{Key: "core.query.calls", Object: queryB, Delta: 20},
				{Key: "mysql.query.temp_disk_tables", Object: queryA, Delta: 12},
			}},
			want: [2]string{"query-a", "query-b"},
		},
		{
			name: "redo pressure",
			rule: redoPressureRule{},
			ctx: finding.AnalysisContext{Deltas: []signal.Delta{
				{Key: "mysql.innodb.log_waits", Object: instanceB, Delta: 6},
				{Key: "mysql.innodb.log_waits", Object: instanceA, Delta: 6},
			}},
			want: [2]string{"instance-a", "instance-b"},
		},
		{
			name: "deadlock rate",
			rule: deadlockRateRule{},
			ctx: finding.AnalysisContext{Deltas: []signal.Delta{
				{Key: "mysql.innodb.deadlocks", Object: instanceB, Delta: 11},
				{Key: "mysql.innodb.deadlocks", Object: instanceA, Delta: 11},
			}},
			want: [2]string{"instance-a", "instance-b"},
		},
		{
			name: "full scan heavy table",
			rule: fullScanHeavyTableRule{},
			ctx: finding.AnalysisContext{
				Current: []signal.Observation{
					signal.NumberObservation("mysql.table.estimated_rows", tableB, 500000, signal.UnitCount, signal.ExactnessEstimated, signal.SensitivityMetadata, time.Time{}),
					signal.NumberObservation("mysql.table.estimated_rows", tableA, 500000, signal.UnitCount, signal.ExactnessEstimated, signal.SensitivityMetadata, time.Time{}),
				},
				Deltas: []signal.Delta{
					{Key: "mysql.table.full_scan_rows", Object: tableB, Delta: 1200000},
					{Key: "mysql.table.full_scan_rows", Object: tableA, Delta: 1200000},
				},
			},
			want: [2]string{"shop.table-a", "shop.table-b"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for run := 0; run < 1000; run++ {
				got := tc.rule.Evaluate(tc.ctx)
				if len(got) != 2 {
					t.Fatalf("run=%d findings=%d want=2", run, len(got))
				}
				if got[0].Object.ID != tc.want[0] || got[1].Object.ID != tc.want[1] {
					t.Fatalf("run=%d finding order=%q,%q want=%q,%q", run, got[0].Object.ID, got[1].Object.ID, tc.want[0], tc.want[1])
				}
			}
		})
	}
}
