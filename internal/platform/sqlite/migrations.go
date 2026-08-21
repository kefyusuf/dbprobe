package sqlite

import "fmt"

const sqliteSchemaVersion = 1

var schemaV1Statements = []string{
	`CREATE TABLE IF NOT EXISTS snapshots (
    id TEXT PRIMARY KEY,
    target_fingerprint TEXT NOT NULL,
    engine TEXT NOT NULL,
    adapter_id TEXT NOT NULL,
    adapter_version TEXT NOT NULL,
    schema_version TEXT NOT NULL,
    collected_at_ns INTEGER NOT NULL,
    payload_json BLOB NOT NULL
)`,
	`CREATE INDEX IF NOT EXISTS snapshots_target_time
    ON snapshots(target_fingerprint, collected_at_ns DESC, id DESC)`,
	`CREATE TABLE IF NOT EXISTS trend_metrics (
    snapshot_id TEXT NOT NULL,
    metric_kind TEXT NOT NULL CHECK(metric_kind IN ('observation', 'delta')),
    signal_key TEXT NOT NULL,
    object_kind TEXT NOT NULL,
    object_id TEXT NOT NULL,
    numeric_value REAL NOT NULL,
    unit TEXT NOT NULL,
    exactness TEXT NOT NULL,
    rate_per_second REAL,
    window_seconds REAL,
    FOREIGN KEY(snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE
)`,
	`CREATE INDEX IF NOT EXISTS trend_metrics_lookup
    ON trend_metrics(signal_key, object_kind, object_id, metric_kind, snapshot_id)`,
}

func bootstrapStatements() []string {
	return []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	}
}

func migrationStatements(currentVersion int) ([]string, error) {
	switch {
	case currentVersion < 0:
		return nil, fmt.Errorf("invalid SQLite schema version %d", currentVersion)
	case currentVersion == sqliteSchemaVersion:
		return []string{}, nil
	case currentVersion > sqliteSchemaVersion:
		return nil, fmt.Errorf("SQLite schema version %d is newer than supported version %d", currentVersion, sqliteSchemaVersion)
	case currentVersion == 0:
		out := append([]string(nil), schemaV1Statements...)
		out = append(out, "PRAGMA user_version = 1")
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported SQLite schema version %d", currentVersion)
	}
}
