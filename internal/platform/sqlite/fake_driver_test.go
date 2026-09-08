package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type fakeSQLiteRecord struct {
	id             string
	target         string
	engine         string
	adapterID      string
	adapterVersion string
	schemaVersion  string
	collected      int64
	payload        []byte
}

type fakeSQLiteState struct {
	mu           sync.Mutex
	userVersion  int64
	pragmas      map[string]int
	ddl          int
	begins       int
	commits      int
	rollbacks    int
	trendInserts int
	failTrend    bool
	snapshots    map[string]fakeSQLiteRecord

	txActive            bool
	pendingSnapshots    map[string]fakeSQLiteRecord
	pendingTrendInserts int
	pendingDDL          int
	pendingUserVersion  *int64
}

func newFakeSQLiteState(version int64) *fakeSQLiteState {
	return &fakeSQLiteState{userVersion: version, pragmas: map[string]int{}, snapshots: map[string]fakeSQLiteRecord{}}
}

var fakeSQLiteDriverSeq atomic.Int64

func openFakeSQLiteDB(t *testing.T, state *fakeSQLiteState) *sql.DB {
	t.Helper()
	name := fmt.Sprintf("dbprobe_sqlite_fake_%d", fakeSQLiteDriverSeq.Add(1))
	sql.Register(name, fakeSQLiteDriver{state: state})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	return db
}

type fakeSQLiteDriver struct{ state *fakeSQLiteState }

func (d fakeSQLiteDriver) Open(string) (driver.Conn, error) {
	return &fakeSQLiteConn{state: d.state}, nil
}

type fakeSQLiteConn struct{ state *fakeSQLiteState }

func (*fakeSQLiteConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}
func (*fakeSQLiteConn) Close() error { return nil }
func (c *fakeSQLiteConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *fakeSQLiteConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.state.mu.Lock()
	defer c.state.mu.Unlock()
	if c.state.txActive {
		return nil, errors.New("nested transaction not supported")
	}
	c.state.begins++
	c.state.txActive = true
	c.state.pendingSnapshots = make(map[string]fakeSQLiteRecord)
	c.state.pendingTrendInserts = 0
	c.state.pendingDDL = 0
	c.state.pendingUserVersion = nil
	return &fakeSQLiteTx{state: c.state}, nil
}
func (c *fakeSQLiteConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.exec(query, args)
}
func (c *fakeSQLiteConn) exec(query string, args []driver.NamedValue) (driver.Result, error) {
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	trim := strings.TrimSpace(query)
	upper := strings.ToUpper(trim)
	if strings.HasPrefix(upper, "PRAGMA FOREIGN_KEYS") || strings.HasPrefix(upper, "PRAGMA BUSY_TIMEOUT") {
		s.pragmas[trim]++
		return driver.RowsAffected(0), nil
	}
	if strings.HasPrefix(upper, "CREATE TABLE") || strings.HasPrefix(upper, "CREATE INDEX") {
		if s.txActive {
			s.pendingDDL++
		} else {
			s.ddl++
		}
		return driver.RowsAffected(0), nil
	}
	if upper == "PRAGMA USER_VERSION = 1" {
		version := int64(1)
		if s.txActive {
			s.pendingUserVersion = &version
		} else {
			s.userVersion = version
		}
		return driver.RowsAffected(0), nil
	}
	if strings.HasPrefix(upper, "INSERT OR IGNORE INTO SNAPSHOTS") {
		id := args[0].Value.(string)
		if _, ok := s.snapshots[id]; ok {
			return driver.RowsAffected(0), nil
		}
		if _, ok := s.pendingSnapshots[id]; ok {
			return driver.RowsAffected(0), nil
		}
		record := fakeSQLiteRecord{
			id:             id,
			target:         args[1].Value.(string),
			engine:         args[2].Value.(string),
			adapterID:      args[3].Value.(string),
			adapterVersion: args[4].Value.(string),
			schemaVersion:  args[5].Value.(string),
			collected:      args[6].Value.(int64),
			payload:        append([]byte(nil), args[7].Value.([]byte)...),
		}
		if s.txActive {
			s.pendingSnapshots[id] = record
		} else {
			s.snapshots[id] = record
		}
		return driver.RowsAffected(1), nil
	}
	if strings.HasPrefix(upper, "INSERT INTO TREND_METRICS") {
		if s.failTrend {
			return nil, errors.New("trend insert failed")
		}
		if s.txActive {
			s.pendingTrendInserts++
		} else {
			s.trendInserts++
		}
		return driver.RowsAffected(1), nil
	}
	return nil, fmt.Errorf("unexpected exec: %s", query)
}
func (c *fakeSQLiteConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	s := c.state
	s.mu.Lock()
	defer s.mu.Unlock()
	upper := strings.ToUpper(strings.TrimSpace(query))
	if upper == "PRAGMA USER_VERSION" {
		return &fakeSQLiteRows{columns: []string{"user_version"}, values: [][]driver.Value{{s.userVersion}}}, nil
	}
	if strings.HasPrefix(upper, "SELECT PAYLOAD_JSON FROM SNAPSHOTS WHERE ID =") {
		id := args[0].Value.(string)
		record, ok := s.snapshots[id]
		if !ok {
			return &fakeSQLiteRows{columns: []string{"payload_json"}}, nil
		}
		return &fakeSQLiteRows{columns: []string{"payload_json"}, values: [][]driver.Value{{append([]byte(nil), record.payload...)}}}, nil
	}
	if strings.Contains(upper, "FROM SNAPSHOTS") && strings.HasPrefix(upper, "SELECT ID, TARGET_FINGERPRINT") {
		target := args[0].Value.(string)
		var before *int64
		if strings.Contains(upper, "COLLECTED_AT_NS <") {
			v := args[1].Value.(int64)
			before = &v
		}
		records := make([]fakeSQLiteRecord, 0)
		for _, r := range s.snapshots {
			if r.target == target && (before == nil || r.collected < *before) {
				records = append(records, r)
			}
		}
		sort.Slice(records, func(i, j int) bool {
			if records[i].collected == records[j].collected {
				return records[i].id > records[j].id
			}
			return records[i].collected > records[j].collected
		})
		if strings.Contains(upper, "LIMIT 1") && len(records) > 1 {
			records = records[:1]
		}
		if strings.Contains(upper, "LIMIT ?") && len(args) > 1 && before == nil {
			limit := int(args[1].Value.(int64))
			if limit >= 0 && limit < len(records) {
				records = records[:limit]
			}
		}
		vals := make([][]driver.Value, 0, len(records))
		for _, r := range records {
			vals = append(vals, []driver.Value{r.id, r.target, r.engine, r.adapterID, r.adapterVersion, r.schemaVersion, r.collected, append([]byte(nil), r.payload...)})
		}
		return &fakeSQLiteRows{columns: []string{"id", "target_fingerprint", "engine", "adapter_id", "adapter_version", "schema_version", "collected_at_ns", "payload_json"}, values: vals}, nil
	}
	return nil, fmt.Errorf("unexpected query: %s", query)
}

type fakeSQLiteTx struct{ state *fakeSQLiteState }

func (tx *fakeSQLiteTx) Commit() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	for id, record := range tx.state.pendingSnapshots {
		tx.state.snapshots[id] = record
	}
	tx.state.trendInserts += tx.state.pendingTrendInserts
	tx.state.ddl += tx.state.pendingDDL
	if tx.state.pendingUserVersion != nil {
		tx.state.userVersion = *tx.state.pendingUserVersion
	}
	tx.state.commits++
	tx.state.clearPending()
	return nil
}
func (tx *fakeSQLiteTx) Rollback() error {
	tx.state.mu.Lock()
	defer tx.state.mu.Unlock()
	tx.state.rollbacks++
	tx.state.clearPending()
	return nil
}

func (s *fakeSQLiteState) clearPending() {
	s.txActive = false
	s.pendingSnapshots = nil
	s.pendingTrendInserts = 0
	s.pendingDDL = 0
	s.pendingUserVersion = nil
}

type fakeSQLiteRows struct {
	columns []string
	values  [][]driver.Value
	pos     int
}

func (r *fakeSQLiteRows) Columns() []string { return r.columns }
func (*fakeSQLiteRows) Close() error        { return nil }
func (r *fakeSQLiteRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.values) {
		return io.EOF
	}
	copy(dest, r.values[r.pos])
	r.pos++
	return nil
}
