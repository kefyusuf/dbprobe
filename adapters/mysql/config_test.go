package mysql_test

import (
	"strings"
	"testing"

	mysqladapter "github.com/kefyusuf/dbprobe/adapters/mysql"
)

func TestParseConfigAcceptsMySQLURIWithoutLeakingPassword(t *testing.T) {
	cfg, err := mysqladapter.ParseConfig("mysql://dbprobe:super-secret@db.example:3307/shop?tls=true")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "db.example" || cfg.Port != "3307" || cfg.Database != "shop" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if cfg.DisplayName != "db.example:3307/shop" {
		t.Fatalf("DisplayName = %q", cfg.DisplayName)
	}
	if strings.Contains(cfg.DisplayName, "super-secret") || strings.Contains(cfg.DisplayName, "dbprobe") {
		t.Fatalf("display name leaks credentials: %q", cfg.DisplayName)
	}
	if !strings.Contains(cfg.DriverDSN, "dbprobe:super-secret@tcp(db.example:3307)/shop") {
		t.Fatalf("DriverDSN = %q", cfg.DriverDSN)
	}
}

func TestParseConfigUsesDefaultPort(t *testing.T) {
	cfg, err := mysqladapter.ParseConfig("mysql://dbprobe:secret@db.example/shop")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "3306" || cfg.DisplayName != "db.example:3306/shop" {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}

func TestParseConfigErrorDoesNotEchoCredentialBearingInput(t *testing.T) {
	raw := "mysql://user:do-not-leak@/"
	_, err := mysqladapter.ParseConfig(raw)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "do-not-leak") || strings.Contains(err.Error(), raw) {
		t.Fatalf("error leaks credentials: %q", err)
	}
}
