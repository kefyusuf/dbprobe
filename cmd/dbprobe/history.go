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

func withHistoryStore(ctx context.Context, path string, factory historyStoreFactory, operation func(temporal.Store) error) (err error) {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("history database path is required")
	}
	if factory == nil {
		return fmt.Errorf("history store factory is required")
	}
	if operation == nil {
		return fmt.Errorf("history operation is required")
	}
	store, err := factory(ctx, path)
	if err != nil {
		return err
	}
	if store == nil {
		return fmt.Errorf("history store factory returned nil")
	}
	defer func() {
		closeErr := store.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("close history store: %w", closeErr)
		}
	}()
	return operation(store)
}
