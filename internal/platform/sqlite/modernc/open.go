package modernc

import (
	"context"
	"database/sql/driver"

	platformsqlite "github.com/kefyusuf/dbprobe/internal/platform/sqlite"
	modernsqlite "modernc.org/sqlite"
)

func Open(ctx context.Context, path string) (*platformsqlite.OwnedStore, error) {
	return platformsqlite.Open(ctx, path, newConnector)
}

func newConnector(path string) (driver.Connector, error) {
	return modernsqlite.NewConnector(path)
}
