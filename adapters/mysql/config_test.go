package mysql

import (
	"errors"
	"strings"
	"testing"
)

func TestParseConfigAcceptsMySQLURIWithoutLeakingPassword(t *testing.T) {
	cfg, err := ParseConfig("mysql://dbprobe:super-secret@db.example:3307/shop?tls=true&timeout=3s&readTimeout=7s")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Host != "db.example" || cfg.Port != "3307" || cfg.Database != "shop" {
		t.Fatalf("unexpected config: host=%q port=%q database=%q", cfg.Host, cfg.Port, cfg.Database)
	}
	if cfg.DisplayName != "db.example:3307/shop" {
		t.Fatalf("DisplayName = %q", cfg.DisplayName)
	}
	if strings.Contains(cfg.DisplayName, "super-secret") || strings.Contains(cfg.DisplayName, "dbprobe") {
		t.Fatalf("display name leaks credentials: %q", cfg.DisplayName)
	}
	if !strings.Contains(cfg.driverDSN, "dbprobe:super-secret@tcp(db.example:3307)/shop") {
		t.Fatal("driver DSN was not constructed as expected")
	}
	if !strings.Contains(cfg.driverDSN, "tls=true") || !strings.Contains(cfg.driverDSN, "timeout=3s") || !strings.Contains(cfg.driverDSN, "readTimeout=7s") {
		t.Fatalf("safe connection options missing from driver DSN")
	}
}

func TestParseConfigUsesDefaultPort(t *testing.T) {
	cfg, err := ParseConfig("mysql://dbprobe:secret@db.example/shop")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Port != "3306" || cfg.DisplayName != "db.example:3306/shop" {
		t.Fatalf("unexpected defaults: host=%q port=%q display=%q", cfg.Host, cfg.Port, cfg.DisplayName)
	}
}

func TestParseConfigRejectsUnsafeOrUnknownOptionsWithoutEchoingInput(t *testing.T) {
	for _, raw := range []string{
		"mysql://user:secret@db.example/shop?multiStatements=true",
		"mysql://user:secret@db.example/shop?allowAllFiles=true",
		"mysql://user:secret@db.example/shop?allowCleartextPasswords=true",
		"mysql://user:secret@db.example/shop?sql_mode=VERY_SECRET_VALUE",
		"mysql://user:secret@db.example/shop?VERY_SECRET_KEY=value",
	} {
		_, err := ParseConfig(raw)
		if err == nil {
			t.Fatalf("expected unsafe option to be rejected: %s", raw)
		}
		for _, forbidden := range []string{"VERY_SECRET_VALUE", "VERY_SECRET_KEY", "secret", raw} {
			if strings.Contains(err.Error(), forbidden) {
				t.Fatalf("option error leaks input %q: %q", forbidden, err)
			}
		}
	}
}

func TestParseConfigErrorDoesNotEchoCredentialBearingInput(t *testing.T) {
	raw := "mysql://user:do-not-leak@/"
	_, err := ParseConfig(raw)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "do-not-leak") || strings.Contains(err.Error(), raw) {
		t.Fatalf("error leaks credentials: %q", err)
	}
}

func TestSanitizeErrorRemovesRawTargetAndPassword(t *testing.T) {
	cfg, err := ParseConfig("mysql://dbprobe:super-secret@db.example:3306/shop")
	if err != nil {
		t.Fatal(err)
	}
	err = sanitizeError(errors.New("dial failed for mysql://dbprobe:super-secret@db.example:3306/shop: super-secret"), cfg)
	if err == nil {
		t.Fatal("expected sanitized error")
	}
	if strings.Contains(err.Error(), "super-secret") || strings.Contains(err.Error(), "dbprobe:") {
		t.Fatalf("sanitized error still leaks credentials: %q", err)
	}
}
