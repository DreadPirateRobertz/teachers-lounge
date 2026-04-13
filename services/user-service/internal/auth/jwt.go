package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/teacherslounge/user-service/internal/models"
)


// Claims holds the JWT payload carried in every access token.
type Claims struct {
	jwt.RegisteredClaims
	UserID      string `json:"uid"`
	Email       string `json:"email"`
	AccountType string `json:"acct"`
	SubStatus   string `json:"sub_status,omitempty"`
}

// JWTManager issues and validates HS256 access tokens for user-service.
type JWTManager struct {
	secret          []byte
	accessDuration  time.Duration
	refreshDuration time.Duration
}

// NewJWTManager creates a JWTManager signing with the given HMAC secret and token durations.
func NewJWTManager(secret string, accessDur, refreshDur time.Duration) *JWTManager {
	return &JWTManager{
		secret:          []byte(secret),
		accessDuration:  accessDur,
		refreshDuration: refreshDur,
	}
}

// Audience is the expected "aud" claim value set on every access token.
// All services that validate tokens must check for this value.
const Audience = "teacherslounge-services"

// IssueAccessToken creates a short-lived JWT (15 min) for API authentication.
func (m *JWTManager) IssueAccessToken(user *models.User, subStatus string) (string, error) {
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessDuration)),
			Issuer:    "teacherslounge-user-service",
			Audience:  jwt.ClaimStrings{Audience},
		},
		UserID:      user.ID.String(),
		Email:       user.Email,
		AccountType: string(user.AccountType),
		SubStatus:   subStatus,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// ValidateAccessToken parses and validates a JWT, returning the claims.
// Validates signature, expiry, and audience claim.
func (m *JWTManager) ValidateAccessToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenStr,
		&Claims{},
		func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return m.secret, nil
		},
		jwt.WithAudience(Audience),
	)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// GenerateRefreshToken creates a cryptographically random opaque refresh token.
// Returns: (raw token for cookie, hashed token for DB storage)
func GenerateRefreshToken() (raw, hashed string, err error) {
	b := make([]byte, 48)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generating refresh token: %w", err)
	}
	raw = base64.URLEncoding.EncodeToString(b)
	hashed = hashToken(raw)
	return raw, hashed, nil
}

// HashToken creates a SHA-256 hash of a token for safe DB storage.
func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// HashTokenPublic exposes the hash for use outside the package (store queries).
func HashToken(raw string) string {
	return hashToken(raw)
}

// GenerateSessionID creates a random session UUID.
func GenerateSessionID() uuid.UUID {
	return uuid.New()
}
