// Package ratelimit implements a Redis-backed token bucket rate limiter.
//
// Each bucket is stored as a Redis hash with two fields:
//   tokens     — current token count (float, stored as string)
//   last_refill — Unix timestamp in seconds when tokens were last updated
//
// On every call the Lua script atomically:
//  1. Reads the current time via redis.call('TIME') so the clock is always
//     sourced from Redis — this keeps the implementation testable with
//     miniredis FastForward without passing wall-clock time from Go.
//  2. Computes elapsed seconds since last_refill
//  3. Adds elapsed * rate tokens (clamped to capacity)
//  4. Consumes one token if available
//  5. Persists the new state with a TTL of 2*capacity/rate seconds
//  6. Returns {1, floor(remaining)} if allowed, {0, 0} if denied
//
// Keys follow the pattern "rl:{bucket}:{userID}" to namespace per-endpoint
// limits independently.
package ratelimit

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter performs per-user, per-bucket rate limiting backed by Redis.
type Limiter struct {
	rdb    redis.Cmdable
	script *redis.Script
}

// New creates a Limiter using the provided Redis client.
func New(rdb redis.Cmdable) *Limiter {
	return &Limiter{
		rdb:    rdb,
		script: redis.NewScript(luaTokenBucket),
	}
}

// Bucket describes the rate limit parameters for one endpoint category.
type Bucket struct {
	// Name is used as the middle segment of the Redis key (e.g. "xp", "quiz_start").
	Name string
	// Capacity is the maximum number of tokens in the bucket (= burst limit).
	Capacity float64
	// Rate is tokens added per second (= sustained request rate).
	Rate float64
}

// Pre-defined buckets for XP and quiz endpoints.
var (
	BucketXP         = Bucket{Name: "xp", Capacity: 10, Rate: 10.0 / 60}         // 10 req/min
	BucketQuizStart  = Bucket{Name: "quiz_start", Capacity: 5, Rate: 5.0 / 60}   // 5 req/min
	BucketQuizAnswer = Bucket{Name: "quiz_answer", Capacity: 20, Rate: 20.0 / 60} // 20 req/min
)

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

// Allow checks whether userID is within the rate limit for the given bucket.
// Returns a Result and a non-nil error only on Redis failure.
func (l *Limiter) Allow(ctx context.Context, b Bucket, userID string) (Result, error) {
	key := fmt.Sprintf("rl:%s:%s", b.Name, userID)
	ttlSec := int(math.Ceil(b.Capacity/b.Rate) * 2)

	vals, err := l.script.Run(ctx, l.rdb,
		[]string{key},
		b.Capacity, b.Rate, ttlSec,
	).Int64Slice()
	if err != nil {
		return Result{}, fmt.Errorf("rate limit script: %w", err)
	}

	allowed := vals[0] == 1
	remaining := int(vals[1])

	var retryAfter time.Duration
	if !allowed {
		// One token costs 1/rate seconds.
		retryAfter = time.Duration(float64(time.Second) / b.Rate)
	}

	return Result{Allowed: allowed, Remaining: remaining, RetryAfter: retryAfter}, nil
}

// luaTokenBucket is the atomic token-bucket Lua script.
//
// Time is read from Redis via TIME so that miniredis FastForward works in
// tests without any wall-clock dependency on the Go side.
//
// KEYS[1]  — Redis hash key
// ARGV[1]  — capacity (float)
// ARGV[2]  — refill rate in tokens/second (float)
// ARGV[3]  — TTL in whole seconds (int)
//
// Returns {1, remaining} if allowed, {0, 0} if denied.
const luaTokenBucket = `
local key        = KEYS[1]
local capacity   = tonumber(ARGV[1])
local rate       = tonumber(ARGV[2])
local ttl        = tonumber(ARGV[3])

local t          = redis.call('TIME')
local now        = tonumber(t[1]) + tonumber(t[2]) / 1000000

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
