// Package auth provides JWT authentication for the notification service.
package auth

import (
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// JWTAuthenticator validates HS256-signed JWTs and extracts the member ID from
// the standard "sub" claim.
type JWTAuthenticator struct {
	secret []byte
}

// NewJWTAuthenticator returns a JWTAuthenticator that validates tokens signed
// with the given HMAC secret. The secret must be at least 16 bytes.
func NewJWTAuthenticator(secret []byte) (*JWTAuthenticator, error) {
	if len(secret) < 16 {
		return nil, errors.New("jwt secret must be at least 16 bytes")
	}
	return &JWTAuthenticator{secret: secret}, nil
}

// MemberID parses and validates tokenStr, returning the "sub" claim value as
// the member ID. Returns an error if the token is invalid, expired, or unsigned
// with the expected algorithm.
func (a *JWTAuthenticator) MemberID(tokenStr string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return "", fmt.Errorf("parse jwt: %w", err)
	}

	sub, err := token.Claims.GetSubject()
	if err != nil || sub == "" {
		return "", errors.New("jwt: missing or empty sub claim")
	}
	return sub, nil
}
