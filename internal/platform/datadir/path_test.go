package datadir

import (
	"path/filepath"
	"testing"
)

type fakeEnv map[string]string

func (e fakeEnv) get(key string) string { return e[key] }

func TestResolveBaseDirByPlatform(t *testing.T) {
	cases := []struct {
		name, goos, home string
		env              fakeEnv
		want             string
	}{
		{name: "linux-xdg", goos: "linux", home: "/home/u", env: fakeEnv{"XDG_DATA_HOME": "/data/u"}, want: "/data/u"},
		{name: "linux-home", goos: "linux", home: "/home/u", env: fakeEnv{}, want: filepath.Join("/home/u", ".local", "share")},
		{name: "linux-relative-xdg-ignored", goos: "linux", home: "/home/u", env: fakeEnv{"XDG_DATA_HOME": "relative"}, want: filepath.Join("/home/u", ".local", "share")},
		{name: "darwin", goos: "darwin", home: "/Users/u", env: fakeEnv{}, want: filepath.Join("/Users/u", "Library", "Application Support")},
		{name: "windows-localappdata", goos: "windows", home: "C:/Users/u", env: fakeEnv{"LOCALAPPDATA": "C:/Users/u/AppData/Local"}, want: "C:/Users/u/AppData/Local"},
		{name: "windows-home-fallback", goos: "windows", home: "C:/Users/u", env: fakeEnv{}, want: filepath.Join("C:/Users/u", "AppData", "Local")},
		{name: "other", goos: "freebsd", home: "/home/u", env: fakeEnv{}, want: filepath.Join("/home/u", ".local", "share")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveBaseDir(tc.goos, tc.env.get, tc.home)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got=%q want=%q", got, tc.want)
			}
		})
	}
}

func TestResolveBaseDirRequiresHomeWhenNoPlatformEnvAvailable(t *testing.T) {
	if _, err := resolveBaseDir("linux", fakeEnv{}.get, ""); err == nil {
		t.Fatal("expected missing home error")
	}
	if _, err := resolveBaseDir("windows", fakeEnv{}.get, ""); err == nil {
		t.Fatal("expected missing home error")
	}
}

func TestBaselineDBPathUsesDbprobeNamespace(t *testing.T) {
	got, err := baselineDBPath("linux", fakeEnv{"XDG_DATA_HOME": "/data/u"}.get, "/home/u")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/data/u", "dbprobe", "dbprobe.db")
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}
