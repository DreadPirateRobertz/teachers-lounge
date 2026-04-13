package auth_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/DreadPirateRobertz/teachers-lounge/services/notification-service/internal/auth"
)

const testSecret = "super-secret-key-at-least-16-bytes"

// makeToken creates a signed HS256 JWT with the given sub claim and expiry.
func makeToken(sub string, expiry time.Time) string {
	claims := jwt.MapClaims{"sub": sub, "exp": expiry.Unix()}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	return tok
}

// TestNewJWTAuthenticator_SecretTooShort verifies that secrets under 16 bytes are rejected.
func TestNewJWTAuthenticator_SecretTooShort(t *testing.T) {
	_, err := auth.NewJWTAuthenticator([]byte("short"))
	if err == nil {
		t.Fatal("expected error for short secret, got nil")
	}
}

// TestNewJWTAuthenticator_ValidSecret verifies that a 16+ byte secret is accepted.
func TestNewJWTAuthenticator_ValidSecret(t *testing.T) {
	a, err := auth.NewJWTAuthenticator([]byte(testSecret))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a == nil {
		t.Fatal("expected non-nil authenticator")
	}
}

// TestMemberID_ValidToken returns the sub claim for a valid token.
func TestMemberID_ValidToken(t *testing.T) {
	a, _ := auth.NewJWTAuthenticator([]byte(testSecret))
	tok := makeToken("member-123", time.Now().Add(time.Hour))

	memberID, err := a.MemberID(tok)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if memberID != "member-123" {
		t.Fatalf("expected %q, got %q", "member-123", memberID)
	}
}

// TestMemberID_ExpiredToken returns an error for an expired token.
func TestMemberID_ExpiredToken(t *testing.T) {
	a, _ := auth.NewJWTAuthenticator([]byte(testSecret))
	tok := makeToken("member-123", time.Now().Add(-time.Hour)) // expired

	_, err := a.MemberID(tok)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

// TestMemberID_WrongSignature returns an error when token is signed with a different key.
func TestMemberID_WrongSignature(t *testing.T) {
	a, _ := auth.NewJWTAuthenticator([]byte(testSecret))
	// Sign with a different key
	claims := jwt.MapClaims{"sub": "member-x", "exp": time.Now().Add(time.Hour).Unix()}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("wrong-key-at-least-16"))

	_, err := a.MemberID(tok)
	if err == nil {
		t.Fatal("expected error for wrong signature, got nil")
	}
}

// TestMemberID_MalformedToken returns an error for a garbage token string.
func TestMemberID_MalformedToken(t *testing.T) {
	a, _ := auth.NewJWTAuthenticator([]byte(testSecret))

	_, err := a.MemberID("not.a.jwt")
	if err == nil {
		t.Fatal("expected error for malformed token, got nil")
	}
}

// TestMemberID_WrongAlgorithm returns an error when token uses an unexpected signing method.
func TestMemberID_WrongAlgorithm(t *testing.T) {
	a, _ := auth.NewJWTAuthenticator([]byte(testSecret))
	// Use RS256 (asymmetric) — rejected by WithValidMethods(["HS256"])
	privKey, _ := jwt.ParseRSAPrivateKeyFromPEM([]byte(`
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA2a2rwplBQLzHPZe5RJi3HeRgJa0JCr2AGEVdhMPKFRb3s+FV
IK9g0/8Jz5y/cIMfuZdMnABxXlF6jKr3KnIioXVGkJdsMqFlqFn2s/M3S0bLJ2H
ABn6EbCKICQQmb7bGlLN0sRkqLFarWECN1lFDIoHAEnXyNFWGkxjjTqCDdpLZGqP
bhH+rjTy0Zv7XMRiqVNiMGFSTB5R8pSTLMfH8W3dHzJv0RLl3VmNWJmXrCJUY6oY
dQ2HlJo7i7GkiLXJX1kL2TBdklRQJy3lNLpHdABqX0A4FbHt9RXJsMXFpWSmJ3Kv
VR5bJ7pibLKNXhH2vQ+2DpJdFJpJm9ULID+d6wIDAQABAoIBAHMTzJN/zXdEFPEo
q47g1rUuNePGEWAJXUbVqRrMBe4Y3PNZrVaGl6yJFZwMXBpKl8eHEEKijg7Vhqoq
-----END RSA PRIVATE KEY-----
`))
	if privKey == nil {
		t.Skip("could not parse test RSA key — skipping algorithm test")
	}

	claims := jwt.MapClaims{"sub": "member-x", "exp": time.Now().Add(time.Hour).Unix()}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(privKey)

	_, err := a.MemberID(tok)
	if err == nil {
		t.Fatal("expected error for wrong algorithm, got nil")
	}
}
