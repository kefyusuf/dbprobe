package mysqlintegration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	mysqladapter "github.com/kefyusuf/dbprobe/adapters/mysql"
	appexplain "github.com/kefyusuf/dbprobe/internal/app/explain"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
)

func TestMySQLPlanExplainMatrixIsEstimatedAndSanitized(t *testing.T) {
	if testing.Short() {
		t.Skip("MySQL integration requires Docker services")
	}
	if !mysqlIntegrationEnabled() {
		t.Skip("set DBPROBE_MYSQL_INTEGRATION=1 after starting the MySQL integration services")
	}

	cases := []targetCase{
		{name: "mysql80", uri: envOr("DBPROBE_MYSQL80_DSN", "mysql://dbprobe:dbprobe-pass@127.0.0.1:13306/shop")},
		{name: "mysql84", uri: envOr("DBPROBE_MYSQL84_DSN", "mysql://dbprobe:dbprobe-pass@127.0.0.1:13307/shop")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			registry, err := adapterregistry.New(mysqladapter.New())
			if err != nil {
				t.Fatal(err)
			}
			statement := "SELECT * FROM shop.customers WHERE email = 'alice@example.test'"
			report, err := appexplain.New(registry).Run(ctx, tc.uri, statement)
			if err != nil {
				t.Fatalf("explain: %v", err)
			}
			if report.SchemaVersion != appexplain.SchemaVersion || report.Target.Engine != "mysql" || report.Format != "mysql-json-sanitized" || !report.Estimated || !report.Sanitized {
				t.Fatalf("unexpected explain report: %#v", report)
			}
			if strings.Contains(report.Plan, "alice@example.test") || strings.Contains(report.Plan, "attached_condition") {
				t.Fatalf("sanitized plan leaked query literal/condition: %s", report.Plan)
			}
			if !strings.Contains(report.Plan, "customers") {
				t.Fatalf("sanitized plan lost table metadata: %s", report.Plan)
			}
		})
	}
}

func mysqlIntegrationEnabled() bool {
	return envOr("DBPROBE_MYSQL_INTEGRATION", "0") == "1"
}
