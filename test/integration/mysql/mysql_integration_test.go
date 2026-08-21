package mysqlintegration_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	mysqladapter "github.com/kefyusuf/dbprobe/adapters/mysql"
	"github.com/kefyusuf/dbprobe/internal/app/inspect"
	"github.com/kefyusuf/dbprobe/internal/core/collection"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	jsonsurface "github.com/kefyusuf/dbprobe/internal/surfaces/json"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type targetCase struct {
	name string
	uri  string
}

func TestMySQLAdapterMatrix(t *testing.T) {
	if os.Getenv("DBPROBE_MYSQL_INTEGRATION") != "1" {
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

			workloadDB := openFixtureDB(t, tc.uri)
			defer workloadDB.Close()
			if err := workloadDB.PingContext(ctx); err != nil {
				t.Fatalf("ping fixture: %v", err)
			}

			for i := 0; i < 25; i++ {
				var name string
				if err := workloadDB.QueryRowContext(ctx, "SELECT name FROM shop.customers WHERE id = ?", 1).Scan(&name); err != nil {
					t.Fatalf("generate read workload: %v", err)
				}
			}
			if _, err := workloadDB.ExecContext(ctx, "INSERT INTO shop.customers(email, name) VALUES ('write-must-fail@example.test', 'forbidden')"); err == nil {
				t.Fatal("dbprobe fixture user unexpectedly has write access")
			}

			a := mysqladapter.New()
			spec, err := adapter.ParseTarget(tc.uri)
			if err != nil {
				t.Fatal(err)
			}
			first, err := a.Open(ctx, spec, adapter.OpenOptions{})
			if err != nil {
				t.Fatalf("open adapter: %v", err)
			}
			defer first.Close()
			second, err := a.Open(ctx, spec, adapter.OpenOptions{})
			if err != nil {
				t.Fatalf("open second adapter: %v", err)
			}
			defer second.Close()

			if first.Target().Engine != "mysql" || first.Target().AdapterID != "mysql" {
				t.Fatalf("unexpected target metadata: %#v", first.Target())
			}
			if first.Target().Fingerprint == "" || first.Target().Fingerprint != second.Target().Fingerprint {
				t.Fatalf("unstable fingerprint: %q vs %q", first.Target().Fingerprint, second.Target().Fingerprint)
			}
			for _, required := range []string{"mysql.performance_schema", "workload.query_summary", "schema.indexes", "schema.objects", "storage.cache", "query.explain", "mysql.innodb"} {
				if !first.Capabilities().Has(requiredCapability(required)) {
					t.Fatalf("missing expected capability %q; got %v", required, first.Capabilities().List())
				}
			}
			if !first.SecurityProfile().ReadOnlyGuaranteed {
				t.Fatal("adapter did not report its read-only execution contract")
			}
			if len(first.Collectors()) < 7 {
				t.Fatalf("collector count = %d, want at least 7", len(first.Collectors()))
			}
			if len(first.Rules()) < 10 {
				t.Fatalf("rule count = %d, want at least 10", len(first.Rules()))
			}

			registry, err := adapterregistry.New(a)
			if err != nil {
				t.Fatal(err)
			}
			service := inspect.New(registry, collection.New(collection.RealWaiter{}, time.Now))
			report, err := service.Run(ctx, tc.uri, 10*time.Millisecond)
			if err != nil {
				t.Fatalf("inspect: %v", err)
			}
			if report.SchemaVersion != inspect.SchemaVersion || report.Target.Engine != "mysql" {
				t.Fatalf("unexpected report contract: %#v", report)
			}
			if !hasObservation(report, "core.connections.limit") {
				t.Fatal("health evidence missing from report")
			}
			if !hasObservation(report, "core.query.calls") {
				t.Fatal("query digest evidence missing after generated workload")
			}

			var rendered bytes.Buffer
			if err := jsonsurface.Render(&rendered, report); err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(rendered.Bytes(), &decoded); err != nil {
				t.Fatalf("invalid inspect JSON: %v", err)
			}
			if decoded["schema_version"] != inspect.SchemaVersion {
				t.Fatalf("JSON schema_version = %#v", decoded["schema_version"])
			}
		})
	}
}

func openFixtureDB(t *testing.T, raw string) *sql.DB {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	password := ""
	if u.User == nil {
		t.Fatal("fixture URI requires credentials")
	}
	password, _ = u.User.Password()
	port := u.Port()
	if port == "" {
		port = "3306"
	}
	cfg := mysqldriver.NewConfig()
	cfg.User = u.User.Username()
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(u.Hostname(), port)
	cfg.DBName = strings.TrimPrefix(u.Path, "/")
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 5 * time.Second
	cfg.WriteTimeout = 5 * time.Second
	connector, err := mysqldriver.NewConnector(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return sql.OpenDB(connector)
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func requiredCapability(value string) interface{ ~string } {
	panic(fmt.Sprintf("unreachable capability conversion for %s", value))
}

func hasObservation(report inspect.Report, key string) bool {
	for _, observation := range report.Observations {
		if string(observation.Key) == key {
			return true
		}
	}
	return false
}
