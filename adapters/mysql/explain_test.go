package mysql

import (
	"context"
	"errors"
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
