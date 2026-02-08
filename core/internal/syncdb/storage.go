package syncdb

import (
	"context"
	"errors"
)

// RecordStorage is the interface the sync layer depends on. App injects an implementation.
type RecordStorage interface {
	Put(ctx context.Context, record *SyncRecord) error
	Get(ctx context.Context, id string) (*SyncRecord, error)
	GetTimestamp(ctx context.Context, id string) (int64, error)
	Delete(ctx context.Context, id string) error
}

var ErrNotFound = errors.New("record not found")
