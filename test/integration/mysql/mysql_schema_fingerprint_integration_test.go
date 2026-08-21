package mysqlintegration_test

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	mysqladapter "github.com/kefyusuf/dbprobe/adapters/mysql"
	"github.com/kefyusuf/dbprobe/internal/app/inspect"
	"github.com/kefyusuf/dbprobe/internal/core/collection"
	"github.com/kefyusuf/dbprobe/internal/platform/adapterregistry"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
	"github.com/kefyusuf/dbprobe/sdk/capability"
)

func TestMySQLSchemaFingerprintMatrixIsStableAndOpaque(t *testing.T) {
	if testing.Short() {
		t.Skip("MySQL integration requires Docker services")
	}
	if envOr("DBPROBE_MYSQL_INTEGRATION", "0") != "1" {
		t.Skip("set DBPROBE_MYSQL_INTEGRATION=1 after starting the MySQL integration services")
	}

	cases := []targetCase{
		{name: "mysql80", uri: envOr("DBPROBE_MYSQL80_DSN", "mysql://dbprobe:dbprobe-pass@127.0.0.1:13306/shop")},
		{name: "mysql84", uri: envOr("DBPROBE_MYSQL84_DSN", "mysql://dbprobe:dbprobe-pass@127.0.0.1:13307/shop")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			mysql := mysqladapter.New()
			spec, err := adapter.ParseTarget(tc.uri)
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := mysql.Open(ctx, spec, adapter.OpenOptions{})
			if err != nil {
				t.Fatalf("open adapter: %v", err)
			}
			if !runtime.Capabilities().Has(capability.Capability("mysql.schema_fingerprint")) {
				_ = runtime.Close()
				t.Fatalf("missing mysql.schema_fingerprint capability: %v", runtime.Capabilities().List())
			}
			_ = runtime.Close()

			registry, err := adapterregistry.New(mysql)
			if err != nil {
				t.Fatal(err)
			}
			service := inspect.New(registry, collection.New(collection.RealWaiter{}, time.Now))
			first, err := service.Run(ctx, tc.uri, 10*time.Millisecond)
			if err != nil {
				t.Fatalf("first inspect: %v", err)
			}
			second, err := service.Run(ctx, tc.uri, 10*time.Millisecond)
			if err != nil {
				t.Fatalf("second inspect: %v", err)
			}

			firstFingerprint := requireSchemaFingerprint(t, first)
			secondFingerprint := requireSchemaFingerprint(t, second)
			if firstFingerprint != secondFingerprint {
				t.Fatalf("schema fingerprint changed without schema mutation: %q vs %q", firstFingerprint, secondFingerprint)
			}
		})
	}
}

func requireSchemaFingerprint(t *testing.T, report inspect.Report) string {
	t.Helper()
	count := 0
	fingerprint := ""
	for _, observation := range report.Observations {
		if observation.Key != "mysql.schema.structural_fingerprint" {
			continue
		}
		count++
		if observation.Object.Kind != "mysql.schema" || observation.Object.ID != "shop" || observation.Text == nil {
			t.Fatalf("invalid schema fingerprint observation: %#v", observation)
		}
		fingerprint = *observation.Text
	}
	if count != 1 {
		t.Fatalf("schema fingerprint observation count=%d", count)
	}
	const prefix = "v1:sha256:"
	if !strings.HasPrefix(fingerprint, prefix) {
		t.Fatalf("schema fingerprint=%q", fingerprint)
	}
	digest := strings.TrimPrefix(fingerprint, prefix)
	if len(digest) != 64 {
		t.Fatalf("schema fingerprint digest length=%d", len(digest))
	}
	if _, err := hex.DecodeString(digest); err != nil {
		t.Fatalf("schema fingerprint is not lowercase/valid hex: %q", digest)
	}
	if digest != strings.ToLower(digest) {
		t.Fatalf("schema fingerprint digest is not lowercase: %q", digest)
	}
	return fingerprint
}
