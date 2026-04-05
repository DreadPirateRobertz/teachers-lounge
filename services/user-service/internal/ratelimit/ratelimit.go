// Package ratelimit implements a Redis-backed token bucket rate limiter for the
// user service.  It mirrors the gaming-service implementation, sharing the same
// Lua script and key scheme so both services behave consistently.
//
// Keys follow the pattern "rl:{bucket}:{subject}" where subject is an IP address
// for unauthenticated endpoints (e.g. /auth/register) or a user ID for
// authenticated endpoints.
package ratelimit

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter performs token-bucket rate limiting backed by Redis.
//
// NowFunc returns the current Unix timestamp in seconds (fractional).
// Override in tests to control time deterministically.
type Limiter struct {
	rdb    redis.Cmdable
	script *redis.Script
	// NowFunc returns the current Unix time in seconds. Override in tests.
	NowFunc func() float64
}

// New creates a Limiter using the provided Redis client.
func New(rdb redis.Cmdable) *Limiter {
	return &Limiter{
		rdb:     rdb,
		script:  redis.NewScript(luaTokenBucket),
		NowFunc: func() float64 { return float64(time.Now().UnixNano()) / 1e9 },
	}
}

// Bucket describes the rate limit parameters for one endpoint.
type Bucket struct {
	// Name is used as the middle segment of the Redis key (e.g. "user_create").
	Name string
	// Capacity is the maximum number of tokens (burst limit).
	Capacity float64
	// Rate is tokens added per second (sustained request rate).
	Rate float64
}

// BucketUserCreate limits POST /auth/register to 5 registrations per hour per IP.
// Burst capacity of 5 prevents scripted account creation while allowing a user to
// retry after validation errors.
var BucketUserCreate = Bucket{Name: "user_create", Capacity: 5, Rate: 5.0 / 3600} // 5 req/hour

// Result holds the outcome of an Allow call.
type Result struct {
	// Allowed is true when the request may proceed.
	Allowed bool
	// Remaining is the number of tokens left after this request (floor).
	Remaining int
	// RetryAfter is the minimum wait before the next request will be allowed.
	// Zero when Allowed is true.
	RetryAfter time.Duration
}

// Allow checks whether subject (IP or user ID) is within the rate limit for bucket b.
// Returns a non-nil error only on Redis failure.
func (l *Limiter) Allow(ctx context.Context, b Bucket, subject string) (Result, error) {
	key := fmt.Sprintf("rl:%s:%s", b.Name, subject)
	ttlSec := int(math.Ceil(b.Capacity/b.Rate) * 2)
	now := l.NowFunc()

	vals, err := l.script.Run(ctx, l.rdb,
		[]string{key},
		b.Capacity, b.Rate, ttlSec, now,
	).Int64Slice()
	if err != nil {
		return Result{}, fmt.Errorf("rate limit script: %w", err)
	}

	allowed := vals[0] == 1
	remaining := int(vals[1])

	var retryAfter time.Duration
	if !allowed {
		retryAfter = time.Duration(float64(time.Second) / b.Rate)
	}

	return Result{Allowed: allowed, Remaining: remaining, RetryAfter: retryAfter}, nil
}

// luaTokenBucket is the atomic token-bucket Lua script.
//
// KEYS[1]  — Redis hash key
// ARGV[1]  — capacity (float)
// ARGV[2]  — refill rate in tokens/second (float)
// ARGV[3]  — TTL in whole seconds (int)
// ARGV[4]  — current Unix timestamp in seconds (float, supplied by Go)
//
// Returns {1, remaining} if allowed, {0, 0} if denied.
const luaTokenBucket = `
local key        = KEYS[1]
local capacity   = tonumber(ARGV[1])
local rate       = tonumber(ARGV[2])
local ttl        = tonumber(ARGV[3])
local now        = tonumber(ARGV[4])

local bucket     = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens     = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
    tokens      = capacity
    last_refill = now
end

local elapsed   = math.max(0, now - last_refill)
local refilled  = math.min(capacity, tokens + elapsed * rate)

if refilled < 1 then
    redis.call('HMSET', key, 'tokens', refilled, 'last_refill', now)
    redis.call('EXPIRE', key, ttl)
    return {0, 0}
end

local new_tokens = refilled - 1
redis.call('HMSET', key, 'tokens', new_tokens, 'last_refill', now)
redis.call('EXPIRE', key, ttl)
return {1, math.floor(new_tokens)}
`
