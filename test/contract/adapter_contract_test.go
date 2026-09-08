package contract_test

import (
	"context"
	"os"
	"testing"

	"github.com/kefyusuf/dbprobe/adapters/fake"
	mysqladapter "github.com/kefyusuf/dbprobe/adapters/mysql"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type adapterCase struct {
	name    string
	adapter adapter.Adapter
	target  string
}

func TestAdapterContract(t *testing.T) {
	cases := []adapterCase{
		{name: "fake", adapter: fake.New(), target: "fake://local"},
	}
	if target := os.Getenv("DBPROBE_TEST_MYSQL_DSN"); target != "" {
		cases = append(cases, adapterCase{name: "mysql", adapter: mysqladapter.New(), target: target})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta := tc.adapter.Metadata()
			if meta.ID == "" || meta.Name == "" || meta.Version == "" || meta.ContractVersion == "" {
				t.Fatalf("incomplete metadata: %#v", meta)
			}
			if meta.ContractVersion != adapter.ContractVersion {
				t.Fatalf("contract = %q; want %q", meta.ContractVersion, adapter.ContractVersion)
			}
			spec, err := adapter.ParseTarget(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			first, err := tc.adapter.Open(context.Background(), spec, adapter.OpenOptions{})
			if err != nil {
				t.Fatal(err)
			}
			second, err := tc.adapter.Open(context.Background(), spec, adapter.OpenOptions{})
			if err != nil {
				_ = first.Close()
				t.Fatal(err)
			}
			if first.Target().Fingerprint == "" || first.Target().Fingerprint != second.Target().Fingerprint {
				t.Fatalf("unstable fingerprint: %q vs %q", first.Target().Fingerprint, second.Target().Fingerprint)
			}
			ids := map[string]struct{}{}
			for _, c := range first.Collectors() {
				d := c.Descriptor()
				if d.ID == "" {
					t.Fatal("collector ID must not be empty")
				}
				if _, exists := ids[d.ID]; exists {
					t.Fatalf("duplicate collector ID %q", d.ID)
				}
				ids[d.ID] = struct{}{}
				for _, key := range d.Produces {
					if key == "" {
						t.Fatalf("collector %q has empty signal key", d.ID)
					}
				}
				for _, required := range d.Requires {
					if required == "" {
						t.Fatalf("collector %q has empty required capability", d.ID)
					}
				}
			}
			if err := first.Close(); err != nil {
				t.Fatal(err)
			}
			if err := first.Close(); err != nil {
				t.Fatalf("second Close: %v", err)
			}
			if err := second.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
