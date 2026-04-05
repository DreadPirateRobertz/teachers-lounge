package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Client wraps the Redis client with domain-specific helpers.
type Client struct {
	rdb *redis.Client
}

func New(addr, password string, db int) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
	})
	return &Client{rdb: rdb}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Close() error {
	return c.rdb.Close()
}

// ============================================================
// SESSION (user session context stored as Hash)
// ============================================================

type SessionData struct {
	UserID             string `json:"user_id"`
	Email              string `json:"email"`
	SubscriptionStatus string `json:"subscription_status"`
	AccountType        string `json:"account_type"`
	ActiveCourseID     string `json:"active_course_id,omitempty"`
}

func (c *Client) SetSession(ctx context.Context, key string, data *SessionData, ttl time.Duration) error {
	pipe := c.rdb.Pipeline()
	pipe.HSet(ctx, key,
		"user_id", data.UserID,
		"email", data.Email,
		"subscription_status", data.SubscriptionStatus,
		"account_type", data.AccountType,
		"active_course_id", data.ActiveCourseID,
	)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Client) GetSession(ctx context.Context, key string) (*SessionData, error) {
	vals, err := c.rdb.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(vals) == 0 {
		return nil, fmt.Errorf("session not found")
	}
	return &SessionData{
		UserID:             vals["user_id"],
		Email:              vals["email"],
		SubscriptionStatus: vals["subscription_status"],
		AccountType:        vals["account_type"],
		ActiveCourseID:     vals["active_course_id"],
	}, nil
}

func (c *Client) DeleteSession(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

func (c *Client) RefreshSessionTTL(ctx context.Context, key string, ttl time.Duration) error {
	return c.rdb.Expire(ctx, key, ttl).Err()
}

// ============================================================
// REFRESH TOKEN LOCK (prevents concurrent rotation races)
// ============================================================

// AcquireRefreshLock attempts to acquire a distributed lock for token rotation.
// Returns true if the lock was acquired.
func (c *Client) AcquireRefreshLock(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	cmd := c.rdb.SetArgs(ctx, key, value, redis.SetArgs{Mode: "NX", TTL: ttl})
	if err := cmd.Err(); err != nil {
		return false, err
	}
	return cmd.Val() == "OK", nil
}

func (c *Client) ReleaseRefreshLock(ctx context.Context, key string) error {
	return c.rdb.Del(ctx, key).Err()
}

// ============================================================
// LOGIN RATE LIMIT
// ============================================================

// IncrLoginAttempts increments the login attempt counter for an IP.
// Returns the new count.
func (c *Client) IncrLoginAttempts(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

func (c *Client) GetLoginAttempts(ctx context.Context, key string) (int64, error) {
	n, err := c.rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return n, err
}

// ============================================================
// GENERIC HELPERS (used by other services)
// ============================================================

func (c *Client) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, b, ttl).Err()
}

func (c *Client) Get(ctx context.Context, key string, dest any) error {
	b, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return ErrCacheMiss
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

func (c *Client) Delete(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

// DeleteUserKeys removes all Redis keys scoped to a user ID.
// Deletes fixed keys (session, streak, daily quests, xp today) and
// scans for wildcard boss-state keys (boss:{userID}:*).
func (c *Client) DeleteUserKeys(ctx context.Context, userID string) error {
	// Fixed keys
	fixed := []string{
		fmt.Sprintf("session:%s", userID),
		fmt.Sprintf("streak:%s", userID),
		fmt.Sprintf("quests:daily:%s", userID),
		fmt.Sprintf("xp:today:%s", userID),
		fmt.Sprintf("quotes:seen:%s", userID),
		fmt.Sprintf("ratelimit:%s:notif", userID),
	}
	if err := c.rdb.Del(ctx, fixed...).Err(); err != nil {
		return err
	}

	// Wildcard boss-state keys: boss:{userID}:*
	pattern := fmt.Sprintf("boss:%s:*", userID)
	var cursor uint64
	for {
		keys, next, err := c.rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}

// IncrWithTTL atomically increments key and sets TTL if the key is new.
func (c *Client) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

var ErrCacheMiss = fmt.Errorf("cache miss")
