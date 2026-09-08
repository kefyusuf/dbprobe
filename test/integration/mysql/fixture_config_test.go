package mysqlintegration_test

import "testing"

func TestFixtureDriverConfigPreservesValidatedTLSMode(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "remote tls", raw: "mysql://dbprobe:secret@db.example:3306/shop?tls=true", want: "true"},
		{name: "loopback plaintext", raw: "mysql://dbprobe:secret@127.0.0.1:3306/shop?tls=false", want: "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := fixtureDriverConfig(tc.raw)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.TLSConfig != tc.want {
				t.Fatalf("TLSConfig=%q want=%q", cfg.TLSConfig, tc.want)
			}
		})
	}
}

func TestFixtureDriverConfigRejectsUnsafeTLSOverride(t *testing.T) {
	for _, raw := range []string{
		"mysql://dbprobe:secret@db.example:3306/shop",
		"mysql://dbprobe:secret@db.example:3306/shop?tls=false",
		"mysql://dbprobe:secret@db.example:3306/shop?tls=skip-verify",
	} {
		if _, err := fixtureDriverConfig(raw); err == nil {
			t.Fatalf("expected fixture config to reject unsafe transport: %q", raw)
		}
	}
}
