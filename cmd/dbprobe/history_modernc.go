package main

import (
	"context"

	modernchistory "github.com/kefyusuf/dbprobe/internal/platform/sqlite/modernc"
)

func defaultCommandDependencies() commandDependencies {
	return commandDependencies{
		historyPath: defaultHistoryPath,
		openHistory: func(ctx context.Context, path string) (ownedHistoryStore, error) {
			return modernchistory.Open(ctx, path)
		},
	}
}
