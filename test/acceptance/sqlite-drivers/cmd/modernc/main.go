package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/kefyusuf/dbprobe/test/acceptance/sqlite-drivers/probe"
	modernsqlite "modernc.org/sqlite"
)

func main() {
	result, err := probe.Run("modernc.org/sqlite", modernsqlite.NewConnector, snapshotCount())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func snapshotCount() int {
	value := os.Getenv("DBPROBE_SQLITE_COMPARE_SNAPSHOTS")
	if value == "" {
		return 250
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 2 {
		fmt.Fprintf(os.Stderr, "invalid DBPROBE_SQLITE_COMPARE_SNAPSHOTS %q\n", value)
		os.Exit(2)
	}
	return count
}
