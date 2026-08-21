package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type fakeExplainExecutor struct {
	query string
	plan  string
	err   error
	calls int
}

func (e *fakeExplainExecutor) ExplainJSON(_ context.Context, query string) (string, error) {
	e.calls++
	e.query = query
	if e.err != nil {
		return "", e.err
	}
	return e.plan, nil
}

func TestValidateExplainStatementAllowsConservativeSingleSelect(t *testing.T) {
	got, err := validateExplainStatement("  SELECT * FROM orders WHERE id = 1  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "SELECT * FROM orders WHERE id = 1" {
		t.Fatalf("got=%q", got)
	}
}

func TestValidateExplainStatementRejectsUnsafeShapes(t *testing.T) {
	cases := []string{
		"",
		"EXPLAIN SELECT 1",
		"EXPLAIN ANALYZE SELECT 1",
		"WITH c AS (SELECT 1) SELECT * FROM c",
		"UPDATE t SET x=1",
		"DELETE FROM t",
		"INSERT INTO t VALUES (1)",
		"REPLACE INTO t VALUES (1)",
		"CALL p()",
		"SET @x=1",
		"SELECT 1; SELECT 2",
		"SELECTED FROM t",
		"SELECT \x00 1",
		"SELECT * FROM t FOR UPDATE",
		"SELECT * FROM t FOR SHARE",
		"SELECT * FROM t LOCK IN SHARE MODE",
		"SELECT 1 INTO @x",
		"SELECT 1 INTO OUTFILE '/tmp/x'",
		"SELECT @x := 1",
		"SELECT " + strings.Repeat("x", maxExplainStatementBytes),
	}
	for _, input := range cases {
		if _, err := validateExplainStatement(input); err == nil {
			t.Fatalf("accepted unsafe input %q", input)
		}
	}
}

func TestExplainWithExecutorPrependsOnlyPlanOnlyExplain(t *testing.T) {
	executor := &fakeExplainExecutor{plan: `{"query_block":{"select_id":1}}`}
	got, err := explainWithExecutor(context.Background(), executor, adapter.ExplainRequest{Statement: " SELECT 1 "})
	if err != nil {
		t.Fatal(err)
	}
	if executor.query != "EXPLAIN FORMAT=JSON SELECT 1" {
		t.Fatalf("query=%q", executor.query)
	}
	if got.Engine != "mysql" || got.Format != "mysql-json" || !got.Estimated || got.Plan != executor.plan {
		t.Fatalf("result=%#v", got)
	}
}

func TestExplainWithExecutorRejectsBeforeDatabaseAccessAndSanitizesExecutorErrors(t *testing.T) {
	executor := &fakeExplainExecutor{err: errors.New("syntax error near secret_customer_email")}
	if _, err := explainWithExecutor(context.Background(), executor, adapter.ExplainRequest{Statement: "UPDATE secret SET x=1"}); err == nil {
		t.Fatal("expected validation error")
	}
	if executor.calls != 0 {
		t.Fatalf("executor calls=%d", executor.calls)
	}

	_, err := explainWithExecutor(context.Background(), executor, adapter.ExplainRequest{Statement: "SELECT * FROM secret_customer_email"})
	if err == nil {
		t.Fatal("expected executor error")
	}
	if strings.Contains(err.Error(), "secret_customer_email") {
		t.Fatalf("error leaked statement context: %q", err)
	}
}

func TestSQLExplainExecutorUsesReadOnlyTransactionAndRollsBack(t *testing.T) {
	state := &recordingSQLState{plan: `{"query_block":{}}`}
	name := "dbprobe_explain_readonly_test"
	sql.Register(name, recordingSQLDriver{state: state})
	db, err := sql.Open(name, "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	plan, err := (sqlExplainExecutor{db: db}).ExplainJSON(context.Background(), "EXPLAIN FORMAT=JSON SELECT 1")
	if err != nil {
		t.Fatal(err)
	}
	if plan != state.plan || !state.readOnly || !state.rolledBack || state.query != "EXPLAIN FORMAT=JSON SELECT 1" {
		t.Fatalf("state=%#v plan=%q", state, plan)
	}
}

type recordingSQLState struct {
	plan       string
	query      string
	readOnly   bool
	rolledBack bool
}

type recordingSQLDriver struct{ state *recordingSQLState }

func (d recordingSQLDriver) Open(string) (driver.Conn, error) {
	return &recordingSQLConn{state: d.state}, nil
}

type recordingSQLConn struct{ state *recordingSQLState }

func (*recordingSQLConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("prepare not supported")
}
func (*recordingSQLConn) Close() error              { return nil }
func (*recordingSQLConn) Begin() (driver.Tx, error) { return nil, errors.New("legacy begin not supported") }
func (c *recordingSQLConn) BeginTx(_ context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.state.readOnly = opts.ReadOnly
	return &recordingSQLTx{state: c.state}, nil
}
func (c *recordingSQLConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	c.state.query = query
	return &recordingSQLRows{plan: c.state.plan}, nil
}

type recordingSQLTx struct{ state *recordingSQLState }

func (*recordingSQLTx) Commit() error { return errors.New("commit must not be used") }
func (tx *recordingSQLTx) Rollback() error {
	tx.state.rolledBack = true
	return nil
}

type recordingSQLRows struct {
	plan string
	done bool
}

func (*recordingSQLRows) Columns() []string { return []string{"EXPLAIN"} }
func (*recordingSQLRows) Close() error      { return nil }
func (r *recordingSQLRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = r.plan
	return nil
}
