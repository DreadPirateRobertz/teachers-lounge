package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/auth"
	"github.com/teacherslounge/user-service/internal/models"
)

const testSecret = "test-jwt-secret-at-least-32-chars!!"

func testManager() *auth.JWTManager {
	return auth.NewJWTManager(testSecret, 15*time.Minute, 30*24*time.Hour)
}

func testUser() *models.User {
	return &models.User{
		ID:          uuid.New(),
		Email:       "test@example.com",
		AccountType: models.AccountTypeStandard,
	}
}

// issueWithAudience mints a token signed with testSecret but with a custom audience,
// bypassing JWTManager so we can test audience mismatch rejection.
func issueWithAudience(t *testing.T, aud string) string {
	t.Helper()
	now := time.Now()
	claims := &auth.Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   uuid.New().String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			Audience:  jwt.ClaimStrings{aud},
		},
		UserID: uuid.New().String(),
		Email:  "bad@example.com",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("signing test token: %v", err)
	}
	return s
}

// TestIssueAccessToken_AudienceClaimSet verifies that issued tokens carry the
// expected "aud" claim so downstream services can enforce audience validation.
func TestIssueAccessToken_AudienceClaimSet(t *testing.T) {
	mgr := testManager()
	tokenStr, err := mgr.IssueAccessToken(testUser(), "active")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	claims, err := mgr.ValidateAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}

	found := false
	for _, a := range claims.Audience {
		if a == auth.Audience {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected aud claim to contain %q, got %v", auth.Audience, claims.Audience)
	}
}

// TestValidateAccessToken_WrongAudience verifies that tokens with a mismatched
// audience are rejected, preventing cross-service token reuse.
func TestValidateAccessToken_WrongAudience(t *testing.T) {
	tokenStr := issueWithAudience(t, "wrong-service")
	_, err := testManager().ValidateAccessToken(tokenStr)
	if err == nil {
		t.Error("expected ValidateAccessToken to reject token with wrong audience, but it succeeded")
	}
}

// TestValidateAccessToken_HappyPath verifies that a freshly issued token validates cleanly.
func TestValidateAccessToken_HappyPath(t *testing.T) {
	mgr := testManager()
	user := testUser()
	tokenStr, err := mgr.IssueAccessToken(user, "trialing")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	claims, err := mgr.ValidateAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}

	if claims.UserID != user.ID.String() {
		t.Errorf("UserID mismatch: got %s, want %s", claims.UserID, user.ID.String())
	}
	if claims.SubStatus != "trialing" {
		t.Errorf("SubStatus mismatch: got %s, want trialing", claims.SubStatus)
	}
}
