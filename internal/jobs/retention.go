package jobs

import (
	"context"
	"time"
)

type MessageRetentionStore interface {
	DeleteMessagesBefore(ctx context.Context, before time.Time) (int64, error)
}

type RetentionJob struct {
	store MessageRetentionStore
	clock func() time.Time
}

func NewRetentionJob(store MessageRetentionStore, clock func() time.Time) *RetentionJob {
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	return &RetentionJob{store: store, clock: clock}
}

func (j *RetentionJob) RunOnce(ctx context.Context, keepFor time.Duration) (int64, error) {
	if keepFor <= 0 {
		return 0, nil
	}
	return j.store.DeleteMessagesBefore(ctx, j.clock().Add(-keepFor))
}
