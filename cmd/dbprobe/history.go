package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type ownedHistoryStore interface {
	temporal.Store
	Close() error
}

type historyStoreFactory func(context.Context, string) (ownedHistoryStore, error)

func defaultHistoryPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return historyPath(dir)
}

func historyPath(configDir string) (string, error) {
	if strings.TrimSpace(configDir) == "" {
		return "", fmt.Errorf("user config directory is required")
	}
	return filepath.Join(configDir, "dbprobe", "history.db"), nil
}

func queryRegressionMetrics(target adapter.TargetMetadata) *temporal.MetricPair {
	switch target.Engine {
	case "mysql":
		return &temporal.MetricPair{CallsKey: "core.query.calls", TotalLatencyKey: "mysql.query.total_latency_ms"}
	default:
		return nil
	}
}
