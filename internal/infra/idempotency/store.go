package idempotency

import "context"

type Store interface {
	Exists(ctx context.Context, key string) (bool, error)
	Save(ctx context.Context, key string) error
}
