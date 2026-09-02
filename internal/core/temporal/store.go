package temporal

import (
	"context"
	"errors"
	"time"
)

var ErrSnapshotNotFound = errors.New("snapshot not found")

type Store interface {
	Save(context.Context, Snapshot) error
	Latest(context.Context, string) (*Snapshot, error)
	Previous(context.Context, string, time.Time) (*Snapshot, error)
	List(context.Context, string, int) ([]Snapshot, error)
}
