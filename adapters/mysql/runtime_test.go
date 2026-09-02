package mysql

import "testing"

func TestBuildTargetUsesServerUUIDForStableFingerprint(t *testing.T) {
	cfg, err := ParseConfig("mysql://dbprobe:secret@db.example:3306/shop")
	if err != nil {
		t.Fatal(err)
	}
	identity := serverIdentity{
		Version:           "8.4.7",
		VersionComment:    "MySQL Community Server - GPL",
		Database:          "shop",
		ServerUUID:        "4a8b2dc1-6c90-11ef-9f88-0242ac120002",
		PerformanceSchema: true,
	}
	target := buildTarget(identity, cfg)
	if target.Engine != "mysql" || target.AdapterID != "mysql" || target.DisplayName != "db.example:3306/shop" {
		t.Fatalf("target = %#v", target)
	}
	want := fingerprint("4a8b2dc1-6c90-11ef-9f88-0242ac120002", "shop", "db.example", "3306")
	if target.Fingerprint != want || target.Fingerprint == "" {
		t.Fatalf("fingerprint = %q, want %q", target.Fingerprint, want)
	}
}

func TestFingerprintFallsBackToEndpointWhenUUIDUnavailable(t *testing.T) {
	got := fingerprint("", "shop", "db.example", "3306")
	want := fingerprint("", "shop", "db.example", "3306")
	other := fingerprint("", "other", "db.example", "3306")
	if got == "" || got != want || got == other {
		t.Fatalf("fallback fingerprint is not stable/scoped: got=%q other=%q", got, other)
	}
}
