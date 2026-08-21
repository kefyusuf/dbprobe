package architecture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMySQLCollectorsDoNotReadSensitiveQueryOrReplicationErrorText(t *testing.T) {
	transactions := mustReadRepoFile(t, "adapters/mysql/collectors/transactions.go")
	if strings.Contains(strings.ToUpper(transactions), "TRX_QUERY") {
		t.Fatal("MySQL transaction collector must not read INNODB_TRX.TRX_QUERY")
	}

	replication := mustReadRepoFile(t, "adapters/mysql/collectors/replication.go")
	if strings.Contains(strings.ToUpper(replication), "LAST_ERROR_MESSAGE") {
		t.Fatal("MySQL replication collector must not read LAST_ERROR_MESSAGE")
	}
}

func TestMySQLExplainProductionPathRemainsPlanOnlyReadOnlyAndSanitized(t *testing.T) {
	explain := mustReadRepoFile(t, "adapters/mysql/explain.go")
	upper := strings.ToUpper(explain)
	if strings.Contains(upper, "EXPLAIN ANALYZE") {
		t.Fatal("production MySQL explain path must never use EXPLAIN ANALYZE")
	}
	if !strings.Contains(upper, "EXPLAIN FORMAT=JSON") {
		t.Fatal("production MySQL explain path must use EXPLAIN FORMAT=JSON")
	}
	if !strings.Contains(explain, "ReadOnly: true") {
		t.Fatal("production MySQL explain path must begin a read-only transaction")
	}
	if !strings.Contains(explain, "sanitizeMySQLJSONPlan") {
		t.Fatal("raw MySQL JSON plans must be sanitized before crossing the adapter boundary")
	}
}

func TestExplainRequestStatementCannotBeJSONSerialized(t *testing.T) {
	contract := mustReadRepoFile(t, "sdk/adapter/explain.go")
	if !strings.Contains(contract, "Statement string `json:\"-\"`") {
		t.Fatal("ExplainRequest.Statement must remain excluded from JSON serialization")
	}
}

func TestMySQLConnectionOptionsRemainExplicitAllowlist(t *testing.T) {
	config := mustReadRepoFile(t, "adapters/mysql/config.go")
	for _, allowed := range []string{"\"tls\"", "\"timeout\"", "\"readTimeout\"", "\"writeTimeout\""} {
		if !strings.Contains(config, allowed) {
			t.Fatalf("expected diagnostic connection option %s is missing from the allowlist", allowed)
		}
	}
	for _, forbidden := range []string{"multiStatements", "allowAllFiles", "allowCleartextPasswords", "allowNativePasswords"} {
		if strings.Contains(config, "\""+forbidden+"\"") {
			t.Fatalf("dangerous/expanded MySQL driver option %q entered the connection allowlist", forbidden)
		}
	}
}

func mustReadRepoFile(t *testing.T, path string) string {
	t.Helper()
	root := filepath.Join("..", "..")
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
