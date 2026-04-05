package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/auth"
	"github.com/teacherslounge/user-service/internal/models"
	"github.com/teacherslounge/user-service/internal/store"
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

// AdminChecker is the minimal store interface RequireAdmin needs.
type AdminChecker interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error)
}

// RequireAdmin rejects requests from users that do not have is_admin = true.
// Must be used after Authenticate.
func RequireAdmin(s AdminChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromCtx(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			user, err := s.GetUserByID(r.Context(), userID)
			if err != nil {
				http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
				return
			}
			if !user.IsAdmin {
				http.Error(w, `{"error":"admin access required"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireTeacherProfile verifies that the authenticated user has a teacher profile.
// Must be used after Authenticate (and typically after RequireSelf).
func RequireTeacherProfile(s store.Storer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := UserIDFromCtx(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			_, err := s.GetTeacherProfile(r.Context(), userID)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					http.Error(w, `{"error":"teacher profile required"}`, http.StatusForbidden)
				} else {
					http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
				}
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

// WithUserIDForTest injects a user ID into a context for unit testing.
// Use only in _test.go files.
func WithUserIDForTest(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKeyUserID{}, userID.String())
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
