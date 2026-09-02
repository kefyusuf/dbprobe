package sqlite_drivers_test

import (
	"testing"

	ncrucesdriver "github.com/ncruces/go-sqlite3/driver"
	"github.com/kefyusuf/dbprobe/test/acceptance/sqlite-drivers/probe"
	modernsqlite "modernc.org/sqlite"
)

func TestCandidateDriversSatisfyTheSamePersistenceContract(t *testing.T) {
	ncruces := &ncrucesdriver.SQLite{}
	cases := []struct {
		name    string
		factory probe.ConnectorFactory
	}{
		{name: "modernc", factory: modernsqlite.NewConnector},
		{name: "ncruces", factory: ncruces.OpenConnector},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := probe.Run(tc.name, tc.factory, 5)
			if err != nil {
				t.Fatal(err)
			}
			if result.Snapshots != 5 || result.DatabaseBytes <= 0 {
				t.Fatalf("result=%#v", result)
			}
		})
	}
}
