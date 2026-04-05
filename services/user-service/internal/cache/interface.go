package cache

import (
	"context"
	"time"
)

// Cacher is the interface the auth and user handlers depend on.
// Implemented by *Client; also implemented by test doubles.
type Cacher interface {
	GetLoginAttempts(ctx context.Context, key string) (int64, error)
	IncrLoginAttempts(ctx context.Context, key string, ttl time.Duration) (int64, error)
	AcquireRefreshLock(ctx context.Context, key, value string, ttl time.Duration) (bool, error)
	ReleaseRefreshLock(ctx context.Context, key string) error
	DeleteSession(ctx context.Context, key string) error
	// DeleteUserKeys removes all user-scoped keys from Redis (called on account deletion).
	DeleteUserKeys(ctx context.Context, userID string) error
	// IncrWithTTL increments a counter and sets a TTL; used for endpoint rate limiting.
	IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// Ensure *Client satisfies Cacher at compile time.
var _ Cacher = (*Client)(nil)
