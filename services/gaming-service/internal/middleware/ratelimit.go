package middleware

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/teacherslounge/gaming-service/internal/ratelimit"
)

// RateLimiter abstracts the Allow call so tests can inject a fake.
type RateLimiter interface {
	Allow(ctx context.Context, b ratelimit.Bucket, userID string) (ratelimit.Result, error)
}

// RateLimit returns a Chi middleware that enforces the given token-bucket limit.
// The user ID is read from the request context (set by Authenticate middleware).
// Returns 429 with Retry-After header when the bucket is exhausted.
func RateLimit(lim RateLimiter, b ratelimit.Bucket, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := UserIDFromContext(r.Context())
			if userID == "" {
				// Authenticate middleware should have already rejected this.
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			res, err := lim.Allow(r.Context(), b, userID)
			if err != nil {
				// Rate limiter unavailable — fail open to avoid blocking legitimate traffic.
				logger.Warn("rate limiter error, failing open",
					zap.String("bucket", b.Name),
					zap.String("user_id", userID),
					zap.Error(err),
				)
				next.ServeHTTP(w, r)
				return
			}

			if !res.Allowed {
				retrySecs := int(math.Ceil(res.RetryAfter.Seconds()))
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retrySecs))
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", b.Capacity))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(res.RetryAfter).Unix()))
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", b.Capacity))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
			next.ServeHTTP(w, r)
		})
	}
}
