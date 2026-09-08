package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/internal/platform/datadir"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type ownedHistoryStore interface {
	temporal.Store
	Close() error
}

type historyStoreFactory func(context.Context, string) (ownedHistoryStore, error)

type commandDependencies struct {
	historyPath func() (string, error)
	openHistory historyStoreFactory
}

func defaultHistoryPath() (string, error) {
	return datadir.BaselineDBPath()
}

func (d commandDependencies) resolveHistoryPath() (string, error) {
	resolver := d.historyPath
	if resolver == nil {
		resolver = defaultHistoryPath
	}
	path, err := resolver()
	if err != nil {
		return "", fmt.Errorf("resolve history database path: %w", err)
	}
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("history database path is required")
	}
	return path, nil
}

func queryRegressionMetrics(target adapter.TargetMetadata) *temporal.MetricPair {
	switch target.Engine {
	case "mysql":
		return &temporal.MetricPair{CallsKey: "core.query.calls", TotalLatencyKey: "mysql.query.total_latency_ms"}
	default:
		return nil
	}
}

func openHistoryStore(ctx context.Context, path string, factory historyStoreFactory) (ownedHistoryStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("history database path is required")
	}
	if factory == nil {
		return nil, fmt.Errorf("history store factory is required")
	}
	store, err := factory(ctx, path)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("history store factory returned nil")
	}
	return store, nil
}

func withOpenedHistoryStore(store ownedHistoryStore, operation func(temporal.Store) error) (err error) {
	if store == nil {
		return fmt.Errorf("history store is required")
	}
	defer func() {
		closeErr := store.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("close history store: %w", closeErr)
		}
	}()
	if operation == nil {
		return fmt.Errorf("history operation is required")
	}
	return operation(store)
}

func withHistoryStore(ctx context.Context, path string, factory historyStoreFactory, operation func(temporal.Store) error) error {
	if operation == nil {
		return fmt.Errorf("history operation is required")
	}
	store, err := openHistoryStore(ctx, path, factory)
	if err != nil {
		return err
	}
	return withOpenedHistoryStore(store, operation)
}
