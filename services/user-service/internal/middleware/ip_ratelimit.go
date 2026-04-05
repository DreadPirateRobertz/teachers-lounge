package middleware

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/teacherslounge/user-service/internal/ratelimit"
)

// IPRateLimiter abstracts the Allow call for IP-keyed rate limiting.
// Satisfied by *ratelimit.Limiter and by test stubs.
type IPRateLimiter interface {
	Allow(ctx context.Context, b ratelimit.Bucket, subject string) (ratelimit.Result, error)
}

// IPRateLimit returns a Chi middleware that enforces a token-bucket rate limit
// keyed by the request's real IP address (populated by chimw.RealIP).
//
// Unlike RateLimit (which requires an authenticated user ID), IPRateLimit works
// on unauthenticated endpoints such as POST /auth/register.  When the bucket is
// exhausted it returns 429 with Retry-After and X-RateLimit-* headers.
// On limiter error it fails open.
func IPRateLimit(lim IPRateLimiter, b ratelimit.Bucket) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr // chimw.RealIP has already resolved the real IP

			res, err := lim.Allow(r.Context(), b, ip)
			if err != nil {
				// Fail open — don't block legitimate registrations if Redis is down.
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
