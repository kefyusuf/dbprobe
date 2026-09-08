package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
	"github.com/kefyusuf/dbprobe/internal/platform/datadir"
	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

func TestDefaultHistoryPathUsesPlatformDataDirectory(t *testing.T) {
	got, err := defaultHistoryPath()
	if err != nil {
		t.Fatal(err)
	}
	want, err := datadir.BaselineDBPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}

func TestQueryRegressionMetricsAreChosenOnlyInCompositionRoot(t *testing.T) {
	mysql := queryRegressionMetrics(adapter.TargetMetadata{Engine: "mysql"})
	if mysql == nil || mysql.CallsKey != "core.query.calls" || mysql.TotalLatencyKey != "mysql.query.total_latency_ms" {
		t.Fatalf("mysql metrics=%#v", mysql)
	}
	if got := queryRegressionMetrics(adapter.TargetMetadata{Engine: "mongodb"}); got != nil {
		t.Fatalf("mongodb metrics=%#v; want nil until production profile is registered", got)
	}
}

type fakeOwnedHistoryStore struct {
	closed   int
	closeErr error
}

func (*fakeOwnedHistoryStore) Save(context.Context, temporal.Snapshot) error { return nil }
func (*fakeOwnedHistoryStore) Latest(context.Context, string) (*temporal.Snapshot, error) {
	return nil, temporal.ErrSnapshotNotFound
}
func (*fakeOwnedHistoryStore) Previous(context.Context, string, time.Time) (*temporal.Snapshot, error) {
	return nil, temporal.ErrSnapshotNotFound
}
func (*fakeOwnedHistoryStore) List(context.Context, string, int) ([]temporal.Snapshot, error) {
	return nil, temporal.ErrSnapshotNotFound
}
func (s *fakeOwnedHistoryStore) Close() error {
	s.closed++
	return s.closeErr
}

func TestWithHistoryStoreOwnsLifecycle(t *testing.T) {
	store := &fakeOwnedHistoryStore{}
	calls := 0
	err := withHistoryStore(context.Background(), "/tmp/history.db", func(_ context.Context, path string) (ownedHistoryStore, error) {
		calls++
		if path != "/tmp/history.db" {
			t.Fatalf("path=%q", path)
		}
		return store, nil
	}, func(got temporal.Store) error {
		if got != store {
			t.Fatalf("store=%#v", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 || store.closed != 1 {
		t.Fatalf("calls=%d closed=%d", calls, store.closed)
	}
}

func TestWithHistoryStorePreservesOperationErrorOverCloseError(t *testing.T) {
	store := &fakeOwnedHistoryStore{closeErr: errors.New("close failed")}
	want := errors.New("operation failed")
	err := withHistoryStore(context.Background(), "/tmp/history.db", func(context.Context, string) (ownedHistoryStore, error) {
		return store, nil
	}, func(temporal.Store) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
	if store.closed != 1 {
		t.Fatalf("closed=%d", store.closed)
	}
}

func TestWithHistoryStoreReturnsCloseErrorAfterSuccessfulOperation(t *testing.T) {
	store := &fakeOwnedHistoryStore{closeErr: errors.New("close failed")}
	err := withHistoryStore(context.Background(), "/tmp/history.db", func(context.Context, string) (ownedHistoryStore, error) {
		return store, nil
	}, func(temporal.Store) error {
		return nil
	})
	if err == nil || err.Error() != "close history store: close failed" {
		t.Fatalf("err=%v", err)
	}
}
