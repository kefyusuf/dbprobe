package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type ConnectorFactory func(string) (driver.Connector, error)

type OwnedStore struct {
	*Store
	db *sql.DB

	closeOnce sync.Once
	closeErr  error
}

func Open(ctx context.Context, path string, factory ConnectorFactory) (*OwnedStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("SQLite baseline path is required")
	}
	if factory == nil {
		return nil, fmt.Errorf("SQLite connector factory is required")
	}
	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o700); err != nil {
		return nil, fmt.Errorf("create SQLite baseline directory: %w", err)
	}
	connector, err := factory(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("create SQLite connector: %w", err)
	}
	if connector == nil {
		return nil, fmt.Errorf("SQLite connector factory returned nil")
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store, err := New(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := os.Stat(cleanPath); err == nil {
		if err := os.Chmod(cleanPath, 0o600); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("secure SQLite baseline file permissions: %w", err)
		}
	} else if !os.IsNotExist(err) {
		_ = db.Close()
		return nil, fmt.Errorf("inspect SQLite baseline file: %w", err)
	}
	return &OwnedStore{Store: store, db: db}, nil
}

func (s *OwnedStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	s.closeOnce.Do(func() { s.closeErr = s.db.Close() })
	return s.closeErr
}
