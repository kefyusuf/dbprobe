package main

import "fmt"

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func versionInfo() string {
	return fmt.Sprintf("%s (commit %s, built %s)", version, commit, buildDate)
}
