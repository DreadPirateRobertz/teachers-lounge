package auth_test

// Additional coverage tests for GenerateRefreshToken, HashToken, and GenerateSessionID.
// These complement the existing jwt_test.go.

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/auth"
	"github.com/teacherslounge/user-service/internal/models"
)

// ── GenerateRefreshToken ──────────────────────────────────────────────────────

// TestGenerateRefreshToken_ReturnsTwoNonEmptyValues verifies that
// GenerateRefreshToken produces a raw token (for the cookie) and a hash
// (for DB storage), both non-empty.
func TestGenerateRefreshToken_ReturnsTwoNonEmptyValues(t *testing.T) {
	raw, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: unexpected error: %v", err)
	}
	if raw == "" {
		t.Error("expected non-empty raw token")
	}
	if hash == "" {
		t.Error("expected non-empty token hash")
	}
}

// TestGenerateRefreshToken_RawAndHashDiffer verifies that the raw token and its
// hash are distinct values — the hash must never be stored in a cookie.
func TestGenerateRefreshToken_RawAndHashDiffer(t *testing.T) {
	raw, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if raw == hash {
		t.Error("raw token and hash must differ")
	}
}

// TestGenerateRefreshToken_ProducesUniqueTokens verifies that two successive
// calls return different raw tokens (random entropy prevents collisions).
func TestGenerateRefreshToken_ProducesUniqueTokens(t *testing.T) {
	raw1, _, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	raw2, _, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if raw1 == raw2 {
		t.Error("two calls to GenerateRefreshToken returned the same raw token — likely broken RNG")
	}
}

// ── HashToken ────────────────────────────────────────────────────────────────

// TestHashToken_IsDeterministic verifies that HashToken produces the same
// output for the same input across multiple calls.
func TestHashToken_IsDeterministic(t *testing.T) {
	input := "some-raw-refresh-token"
	h1 := auth.HashToken(input)
	h2 := auth.HashToken(input)
	if h1 != h2 {
		t.Errorf("HashToken is not deterministic: %q != %q", h1, h2)
	}
}

// TestHashToken_NonEmpty verifies that HashToken always produces a non-empty string.
func TestHashToken_NonEmpty(t *testing.T) {
	if got := auth.HashToken("any-token"); got == "" {
		t.Error("HashToken returned empty string")
	}
}

// TestHashToken_DifferentInputsDifferentOutputs verifies that distinct inputs
// produce distinct hashes (collision resistance for different tokens).
func TestHashToken_DifferentInputsDifferentOutputs(t *testing.T) {
	h1 := auth.HashToken("token-aaa")
	h2 := auth.HashToken("token-bbb")
	if h1 == h2 {
		t.Error("different inputs produced the same hash")
	}
}

// TestHashToken_MatchesExpected verifies that GenerateRefreshToken's hash is
// reproducible by calling HashToken on the raw value.
func TestHashToken_MatchesExpected(t *testing.T) {
	raw, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	if got := auth.HashToken(raw); got != hash {
		t.Errorf("HashToken(raw) = %q; want %q (from GenerateRefreshToken)", got, hash)
	}
}

// ── GenerateSessionID ────────────────────────────────────────────────────────

// TestGenerateSessionID_ReturnsNonNilUUID verifies that GenerateSessionID
// returns a valid non-nil UUID suitable for session tracking.
func TestGenerateSessionID_ReturnsNonNilUUID(t *testing.T) {
	id := auth.GenerateSessionID()
	if id == uuid.Nil {
		t.Error("GenerateSessionID returned uuid.Nil")
	}
}

// TestGenerateSessionID_ReturnsUniqueIDs verifies that two successive calls
// produce distinct IDs (no accidental constant return value).
func TestGenerateSessionID_ReturnsUniqueIDs(t *testing.T) {
	id1 := auth.GenerateSessionID()
	id2 := auth.GenerateSessionID()
	if id1 == id2 {
		t.Error("GenerateSessionID returned the same ID on two calls")
	}
}

// ── IssueAccessToken — claim content ─────────────────────────────────────────

// TestIssueAccessToken_SubClaimMatchesUserID verifies that the JWT "sub" claim
// equals the user's ID, as required for downstream auth middleware.
func TestIssueAccessToken_SubClaimMatchesUserID(t *testing.T) {
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
	if claims.Subject != user.ID.String() {
		t.Errorf("sub claim = %q; want %q", claims.Subject, user.ID.String())
	}
}

// TestIssueAccessToken_EmailClaimSet verifies that the JWT carries the
// user's email so services can display it without a DB roundtrip.
func TestIssueAccessToken_EmailClaimSet(t *testing.T) {
	mgr := testManager()
	user := testUser()
	tokenStr, err := mgr.IssueAccessToken(user, "active")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	claims, err := mgr.ValidateAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims.Email != user.Email {
		t.Errorf("email claim = %q; want %q", claims.Email, user.Email)
	}
}

// TestIssueAccessToken_ExpiryIsInFuture verifies that every issued token has
// an expiry strictly greater than the current time.
func TestIssueAccessToken_ExpiryIsInFuture(t *testing.T) {
	mgr := testManager()
	tokenStr, err := mgr.IssueAccessToken(testUser(), "active")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	claims, err := mgr.ValidateAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if !claims.ExpiresAt.Time.After(time.Now()) {
		t.Errorf("token expiry %v is not in the future", claims.ExpiresAt.Time)
	}
}

// TestIssueAccessToken_AccountTypeClaimSet verifies that the account_type
// custom claim is included in the token.
func TestIssueAccessToken_AccountTypeClaimSet(t *testing.T) {
	mgr := testManager()
	user := &models.User{
		ID:          testUser().ID,
		Email:       "teacher@example.com",
		AccountType: models.AccountTypeMinor,
	}
	tokenStr, err := mgr.IssueAccessToken(user, "active")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	claims, err := mgr.ValidateAccessToken(tokenStr)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if claims.AccountType != string(models.AccountTypeMinor) {
		t.Errorf("acct claim = %q; want %q", claims.AccountType, models.AccountTypeMinor)
	}
}

// ── ValidateAccessToken — error paths ────────────────────────────────────────

// TestValidateAccessToken_ExpiredToken verifies that an expired token is
// rejected, preventing replay attacks with old tokens.
func TestValidateAccessToken_ExpiredToken(t *testing.T) {
	// Issue a token that expires immediately (negative duration).
	mgr := auth.NewJWTManager(testSecret, -1*time.Second, 30*24*time.Hour)
	tokenStr, err := mgr.IssueAccessToken(testUser(), "active")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	// Validate with a manager that has a normal duration — the token is still expired.
	_, err = testManager().ValidateAccessToken(tokenStr)
	if err == nil {
		t.Error("expected ValidateAccessToken to reject an expired token, but it succeeded")
	}
}

// TestValidateAccessToken_TamperedSignature verifies that any modification to
// the token body invalidates the HMAC signature.
func TestValidateAccessToken_TamperedSignature(t *testing.T) {
	mgr := testManager()
	tokenStr, err := mgr.IssueAccessToken(testUser(), "active")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}

	// Append a character to corrupt the signature segment.
	tampered := tokenStr + "X"
	_, err = mgr.ValidateAccessToken(tampered)
	if err == nil {
		t.Error("expected ValidateAccessToken to reject a tampered token, but it succeeded")
	}
}

// TestValidateAccessToken_WrongSecret verifies that a token signed with a
// different secret is rejected.
func TestValidateAccessToken_WrongSecret(t *testing.T) {
	other := auth.NewJWTManager("different-secret-at-least-32-chars!!", 15*time.Minute, 30*24*time.Hour)
	tokenStr, err := other.IssueAccessToken(testUser(), "active")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	_, err = testManager().ValidateAccessToken(tokenStr)
	if err == nil {
		t.Error("expected ValidateAccessToken to reject token signed with wrong secret")
	}
}
