package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// RateLimitBackend is the minimal Redis interface the RateLimit middleware needs.
// Satisfied by *cache.Client (via IncrLoginAttempts) and by test stubs.
type RateLimitBackend interface {
	// IncrWithTTL increments the counter for key and sets TTL on first write.
	// Returns the new count.
	IncrWithTTL(ctx context.Context, key string, window time.Duration) (int64, error)
}

// RateLimit returns a Chi middleware that allows at most limit requests per
// window per authenticated user ID.  When the limit is exceeded it returns
// 429 Too Many Requests.  On backend error it fails open.
func RateLimit(backend RateLimitBackend, prefix string, limit int64, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromCtx(r.Context())
			if !ok {
				// No authenticated user — pass through; Authenticate handles the 401.
				next.ServeHTTP(w, r)
				return
			}

			key := fmt.Sprintf("rl:%s:%s", prefix, userID)
			count, err := backend.IncrWithTTL(r.Context(), key, window)
			if err != nil {
				// Fail open: rate limiter unavailable.
				next.ServeHTTP(w, r)
				return
			}

			if count > limit {
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", window.Seconds()))
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
