// Package auth implements token creation/validation using HMAC-SHA256 JWTs
// (same security model as Paseto v4 local for a single-key setup).
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the payload stored in every token.
type Claims struct {
	UserID      string `json:"user_id"`
	WorkspaceID string `json:"workspace_id"`
	jwt.RegisteredClaims
}

// Maker creates and verifies tokens.
type Maker struct {
	secret []byte
}

// NewMaker returns a Maker using the provided secret key.
func NewMaker(secret string) (*Maker, error) {
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 characters")
	}
	return &Maker{secret: []byte(secret)}, nil
}

// CreateToken issues a signed token valid for the given duration.
func (m *Maker) CreateToken(userID, workspaceID string, duration time.Duration) (string, error) {
	claims := &Claims{
		UserID:      userID,
		WorkspaceID: workspaceID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// VerifyToken parses and validates a token string, returning its claims.
func (m *Maker) VerifyToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
