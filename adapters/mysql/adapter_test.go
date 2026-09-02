package mysql_test

import (
	"testing"

	mysqladapter "github.com/kefyusuf/dbprobe/adapters/mysql"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

func TestAdapterMetadataAndMatch(t *testing.T) {
	a := mysqladapter.New()
	meta := a.Metadata()
	if meta.ID != "mysql" || meta.Name != "MySQL" || meta.ContractVersion != adapter.ContractVersion {
		t.Fatalf("metadata = %#v", meta)
	}
	spec, err := adapter.ParseTarget("mysql://dbprobe:secret@localhost:3306/app")
	if err != nil {
		t.Fatal(err)
	}
	if !a.Match(spec) {
		t.Fatal("expected mysql target to match")
	}
	fake, _ := adapter.ParseTarget("fake://local")
	if a.Match(fake) {
		t.Fatal("mysql adapter matched fake target")
	}
}
