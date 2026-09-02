package main

import (
	"context"
	"database/sql/driver"

	platformsqlite "github.com/kefyusuf/dbprobe/internal/platform/sqlite"
	moderncsqlite "modernc.org/sqlite"
)

func productionCommandDependencies() commandDependencies {
	return commandDependencies{openHistory: openSQLiteHistory}
}

func openSQLiteHistory(ctx context.Context, path string) (ownedHistoryStore, error) {
	return platformsqlite.Open(ctx, path, moderncSQLiteConnector)
}

func moderncSQLiteConnector(path string) (driver.Connector, error) {
	return moderncsqlite.NewConnector(path)
}
