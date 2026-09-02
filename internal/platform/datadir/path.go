package datadir

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type getenvFunc func(string) string

func BaselineDBPath() (string, error) {
	if base, ok := configuredBaseDir(runtime.GOOS, os.Getenv); ok {
		return filepath.Join(base, "dbprobe", "dbprobe.db"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home directory: %w", err)
	}
	return baselineDBPath(runtime.GOOS, os.Getenv, home)
}

func baselineDBPath(goos string, getenv getenvFunc, home string) (string, error) {
	base, err := resolveBaseDir(goos, getenv, home)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "dbprobe", "dbprobe.db"), nil
}

func resolveBaseDir(goos string, getenv getenvFunc, home string) (string, error) {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	if base, ok := configuredBaseDir(goos, getenv); ok {
		return base, nil
	}
	switch goos {
	case "windows":
		if home == "" {
			return "", fmt.Errorf("user home directory is required")
		}
		return filepath.Join(home, "AppData", "Local"), nil
	case "darwin":
		if home == "" {
			return "", fmt.Errorf("user home directory is required")
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	default:
		if home == "" {
			return "", fmt.Errorf("user home directory is required")
		}
		return filepath.Join(home, ".local", "share"), nil
	}
}

func configuredBaseDir(goos string, getenv getenvFunc) (string, bool) {
	if getenv == nil {
		return "", false
	}
	switch goos {
	case "windows":
		if local := getenv("LOCALAPPDATA"); local != "" {
			return local, true
		}
	case "linux":
		if xdg := getenv("XDG_DATA_HOME"); xdg != "" && filepath.IsAbs(xdg) {
			return xdg, true
		}
	}
	return "", false
}
