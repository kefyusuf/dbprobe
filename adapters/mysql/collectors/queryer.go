package collectors

import (
	"context"
	"database/sql"
)

type Rows interface {
	Next() bool
	Scan(dest ...any) error
	Close() error
	Err() error
}

type Queryer interface {
	QueryContext(context.Context, string, ...any) (Rows, error)
}

type sqlQueryer struct{ db *sql.DB }

func NewSQLQueryer(db *sql.DB) Queryer { return sqlQueryer{db: db} }

func (q sqlQueryer) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return q.db.QueryContext(ctx, query, args...)
}
