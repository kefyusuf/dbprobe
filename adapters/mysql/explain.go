package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kefyusuf/dbprobe/sdk/adapter"
)

type explainExecutor interface {
	ExplainJSON(context.Context, string) (string, error)
}

type sqlExplainExecutor struct{ db *sql.DB }

func (e sqlExplainExecutor) ExplainJSON(ctx context.Context, query string) (string, error) {
	var plan string
	if err := e.db.QueryRowContext(ctx, query).Scan(&plan); err != nil {
		return "", err
	}
	return plan, nil
}

func validateExplainStatement(input string) (string, error) {
	statement := strings.TrimSpace(input)
	if statement == "" {
		return "", fmt.Errorf("explain statement is required")
	}
	if strings.ContainsRune(statement, '\x00') || strings.ContainsRune(statement, ';') {
		return "", fmt.Errorf("explain accepts one SELECT statement only")
	}
	if len(statement) < len("SELECT") || !strings.EqualFold(statement[:len("SELECT")], "SELECT") {
		return "", fmt.Errorf("explain accepts SELECT statements only")
	}
	if len(statement) > len("SELECT") {
		r, _ := utf8.DecodeRuneInString(statement[len("SELECT"):])
		if !unicode.IsSpace(r) {
			return "", fmt.Errorf("explain accepts SELECT statements only")
		}
	}
	return statement, nil
}

func explainWithExecutor(ctx context.Context, executor explainExecutor, request adapter.ExplainRequest) (adapter.ExplainResult, error) {
	statement, err := validateExplainStatement(request.Statement)
	if err != nil {
		return adapter.ExplainResult{}, err
	}
	plan, err := executor.ExplainJSON(ctx, "EXPLAIN FORMAT=JSON "+statement)
	if err != nil {
		return adapter.ExplainResult{}, fmt.Errorf("MySQL plan explain failed")
	}
	if strings.TrimSpace(plan) == "" {
		return adapter.ExplainResult{}, fmt.Errorf("MySQL plan explain returned an empty plan")
	}
	return adapter.ExplainResult{Engine: "mysql", Format: "mysql-json", Estimated: true, Plan: plan}, nil
}

func (r *runtime) ExplainPlan(ctx context.Context, request adapter.ExplainRequest) (adapter.ExplainResult, error) {
	return explainWithExecutor(ctx, sqlExplainExecutor{db: r.db}, request)
}

var _ adapter.PlanExplainer = (*runtime)(nil)
