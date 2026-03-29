package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/auth"
)

type ctxKeyUserID struct{}
type ctxKeyClaims struct{}

// Authenticate validates the Bearer token in Authorization header.
// On success, injects user ID and claims into the request context.
func Authenticate(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}
			claims, err := jwtManager.ValidateAccessToken(token)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKeyUserID{}, claims.UserID)
			ctx = context.WithValue(ctx, ctxKeyClaims{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireActiveSubscription rejects requests from users without an active/trialing subscription.
func RequireActiveSubscription(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromCtx(r.Context())
		if claims == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		switch claims.SubStatus {
		case "trialing", "active":
			next.ServeHTTP(w, r)
		case "past_due":
			http.Error(w, `{"error":"payment past due — please update payment method"}`, http.StatusPaymentRequired)
		default:
			http.Error(w, `{"error":"active subscription required"}`, http.StatusPaymentRequired)
		}
	})
}

// RequireSelf enforces that a user can only access their own resources.
// Expects the route to use {id} parameter via chi.
func RequireSelf(getUserID func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resourceUserID := getUserID(r)
			claims := ClaimsFromCtx(r.Context())
			if claims == nil || claims.UserID != resourceUserID {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ============================================================
// CONTEXT HELPERS
// ============================================================

func UserIDFromCtx(ctx context.Context) (uuid.UUID, bool) {
	raw, ok := ctx.Value(ctxKeyUserID{}).(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	return id, err == nil
}

func ClaimsFromCtx(ctx context.Context) *auth.Claims {
	c, _ := ctx.Value(ctxKeyClaims{}).(*auth.Claims)
	return c
}

// ============================================================
// HELPERS
// ============================================================

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}
