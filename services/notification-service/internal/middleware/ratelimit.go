package middleware

import (
	"context"
	"fmt"
	"net/http"
)

// Limiter is the interface the rate-limit middleware depends on.
type Limiter interface {
	Allow(ctx context.Context, userID string) (bool, error)
}

// PushRateLimit is HTTP middleware that enforces push notification rate limits.
// It expects the request to carry a "user_id" form/query param or be injected
// by the auth middleware into the context (key: "userID").
//
// If the limit is exceeded it writes 429 Too Many Requests and stops the chain.
func PushRateLimit(limiter Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := userIDFromContext(r.Context())
			if userID == "" {
				http.Error(w, "missing user_id", http.StatusBadRequest)
				return
			}

			allowed, err := limiter.Allow(r.Context(), userID)
			if err != nil {
				// Redis failure — fail open so a Redis outage doesn't block all notifications.
				// The stub logs; real implementation should also alert.
				next.ServeHTTP(w, r)
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", fmt.Sprintf("%d", 86400))
				w.Header().Set("X-RateLimit-Limit", "3")
				w.Header().Set("X-RateLimit-Remaining", "0")
				http.Error(w, "push notification rate limit exceeded (3 per 24h)", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type contextKey string

const userIDKey contextKey = "userID"

// WithUserID injects a user ID into the request context.
// Called by the auth middleware (stub) before reaching rate-limit middleware.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func userIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}
