package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kefyusuf/dbprobe/internal/core/temporal"
)

const insertSnapshotSQL = `INSERT OR IGNORE INTO snapshots (
    id, target_fingerprint, engine, adapter_id, adapter_version, schema_version, collected_at_ns, payload_json
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

const insertTrendMetricSQL = `INSERT INTO trend_metrics (
    snapshot_id, metric_kind, signal_key, object_kind, object_id, numeric_value, unit, exactness, rate_per_second, window_seconds
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const snapshotPayloadByIDSQL = `SELECT payload_json FROM snapshots WHERE id = ? LIMIT 1`

const snapshotEnvelopeColumns = `id, target_fingerprint, engine, adapter_id, adapter_version, schema_version, collected_at_ns, payload_json`

const latestSnapshotSQL = `SELECT ` + snapshotEnvelopeColumns + `
FROM snapshots
WHERE target_fingerprint = ?
ORDER BY collected_at_ns DESC, id DESC
LIMIT 1`

const previousSnapshotSQL = `SELECT ` + snapshotEnvelopeColumns + `
FROM snapshots
WHERE target_fingerprint = ? AND collected_at_ns < ?
ORDER BY collected_at_ns DESC, id DESC
LIMIT 1`

const listSnapshotsSQL = `SELECT ` + snapshotEnvelopeColumns + `
FROM snapshots
WHERE target_fingerprint = ?
ORDER BY collected_at_ns DESC, id DESC`

const listSnapshotsLimitedSQL = listSnapshotsSQL + `
LIMIT ?`

type Store struct {
	db *sql.DB
}

func New(ctx context.Context, db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("SQLite database is required")
	}
	store := &Store{db: db}
	if err := store.withConn(ctx, func(conn *sql.Conn) error {
		return migrate(ctx, conn)
	}); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Save(ctx context.Context, snapshot temporal.Snapshot) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("SQLite baseline store is not initialized")
	}
	normalized, err := validateSnapshot(snapshot)
	if err != nil {
		return err
	}
	payload, err := encodeSnapshot(normalized)
	if err != nil {
		return err
	}
	trends, err := extractTrendMetrics(normalized)
	if err != nil {
		return err
	}

	return s.withConn(ctx, func(conn *sql.Conn) error {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin SQLite snapshot transaction: %w", err)
		}
		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		result, err := tx.ExecContext(ctx, insertSnapshotSQL,
			normalized.ID,
			normalized.TargetFingerprint,
			normalized.Engine,
			normalized.AdapterID,
			normalized.AdapterVersion,
			normalized.SchemaVersion,
			normalized.CollectedAt.UnixNano(),
			payload,
		)
		if err != nil {
			return fmt.Errorf("insert SQLite snapshot: %w", err)
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read SQLite snapshot insert result: %w", err)
		}
		if inserted > 1 {
			return fmt.Errorf("SQLite snapshot insert affected %d rows", inserted)
		}
		if inserted == 0 {
			var existingPayload []byte
			if err := tx.QueryRowContext(ctx, snapshotPayloadByIDSQL, normalized.ID).Scan(&existingPayload); err != nil {
				return fmt.Errorf("verify existing SQLite snapshot: %w", err)
			}
			if !bytes.Equal(existingPayload, payload) {
				return fmt.Errorf("SQLite snapshot identity collision: existing payload differs")
			}
		}
		if inserted == 1 {
			for _, trend := range trends {
				var rate any
				if trend.RatePerSecond != nil {
					rate = *trend.RatePerSecond
				}
				var window any
				if trend.WindowSeconds != nil {
					window = *trend.WindowSeconds
				}
				if _, err := tx.ExecContext(ctx, insertTrendMetricSQL,
					trend.SnapshotID,
					string(trend.MetricKind),
					string(trend.SignalKey),
					trend.ObjectKind,
					trend.ObjectID,
					trend.NumericValue,
					string(trend.Unit),
					string(trend.Exactness),
					rate,
					window,
				); err != nil {
					return fmt.Errorf("insert SQLite trend metric: %w", err)
				}
			}
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit SQLite snapshot transaction: %w", err)
		}
		committed = true
		return nil
	})
}

func (s *Store) Latest(ctx context.Context, target string) (*temporal.Snapshot, error) {
	if target == "" {
		return nil, fmt.Errorf("target fingerprint is required")
	}
	var snapshot temporal.Snapshot
	err := s.withConn(ctx, func(conn *sql.Conn) error {
		loaded, err := scanStoredSnapshot(conn.QueryRowContext(ctx, latestSnapshotSQL, target), target)
		if err != nil {
			return err
		}
		snapshot = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (s *Store) Previous(ctx context.Context, target string, before time.Time) (*temporal.Snapshot, error) {
	if target == "" {
		return nil, fmt.Errorf("target fingerprint is required")
	}
	if before.IsZero() {
		return nil, fmt.Errorf("previous snapshot time is required")
	}
	var snapshot temporal.Snapshot
	err := s.withConn(ctx, func(conn *sql.Conn) error {
		loaded, err := scanStoredSnapshot(conn.QueryRowContext(ctx, previousSnapshotSQL, target, before.UTC().UnixNano()), target)
		if err != nil {
			return err
		}
		snapshot = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (s *Store) List(ctx context.Context, target string, limit int) ([]temporal.Snapshot, error) {
	if target == "" {
		return nil, fmt.Errorf("target fingerprint is required")
	}
	var snapshots []temporal.Snapshot
	err := s.withConn(ctx, func(conn *sql.Conn) error {
		var (
			rows *sql.Rows
			err  error
		)
		if limit > 0 {
			rows, err = conn.QueryContext(ctx, listSnapshotsLimitedSQL, target, limit)
		} else {
			rows, err = conn.QueryContext(ctx, listSnapshotsSQL, target)
		}
		if err != nil {
			return fmt.Errorf("query SQLite snapshots: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			snapshot, err := scanStoredSnapshot(rows, target)
			if err != nil {
				return err
			}
			snapshots = append(snapshots, snapshot)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate SQLite snapshots: %w", err)
		}
		if len(snapshots) == 0 {
			return temporal.ErrSnapshotNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshots, nil
}

type snapshotScanner interface {
	Scan(...any) error
}

func scanStoredSnapshot(scanner snapshotScanner, expectedTarget string) (temporal.Snapshot, error) {
	var (
		id, target, engine, adapterID, adapterVersion, schemaVersion string
		collectedAtNS                                                int64
		payload                                                      []byte
	)
	if err := scanner.Scan(&id, &target, &engine, &adapterID, &adapterVersion, &schemaVersion, &collectedAtNS, &payload); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return temporal.Snapshot{}, temporal.ErrSnapshotNotFound
		}
		return temporal.Snapshot{}, fmt.Errorf("scan SQLite snapshot: %w", err)
	}
	snapshot, err := decodeSnapshot(payload)
	if err != nil {
		return temporal.Snapshot{}, err
	}
	if target != expectedTarget || snapshot.TargetFingerprint != target || snapshot.ID != id || snapshot.Engine != engine || snapshot.AdapterID != adapterID || snapshot.AdapterVersion != adapterVersion || snapshot.SchemaVersion != schemaVersion || snapshot.CollectedAt.UnixNano() != collectedAtNS {
		return temporal.Snapshot{}, fmt.Errorf("SQLite snapshot envelope does not match payload")
	}
	return snapshot, nil
}

func (s *Store) withConn(ctx context.Context, fn func(*sql.Conn) error) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("SQLite baseline store is not initialized")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire SQLite connection: %w", err)
	}
	defer conn.Close()
	if err := prepareConnection(ctx, conn); err != nil {
		return err
	}
	return fn(conn)
}

var _ temporal.Store = (*Store)(nil)
