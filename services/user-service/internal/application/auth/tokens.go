// Package auth contains authentication use cases and JWT token management.
package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	accessTokenDuration  = time.Hour
	refreshTokenDuration = 7 * 24 * time.Hour
	tokenTypeAccess      = "access"
	tokenTypeRefresh     = "refresh"
)

// Claims is the JWT payload for access tokens.
type Claims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

// TokenPair holds both access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int64  `json:"expiresIn"` // access token TTL in seconds
}

func jwtSecret() []byte {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		panic("JWT_SECRET env var is required")
	}
	return []byte(s)
}

// IssueTokenPair generates a new access + refresh token pair for a user.
func IssueTokenPair(userID, email, role string) (*TokenPair, error) {
	accessToken, err := issueToken(userID, email, role, tokenTypeAccess, accessTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}

	refreshToken, err := issueToken(userID, email, role, tokenTypeRefresh, refreshTokenDuration)
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(accessTokenDuration.Seconds()),
	}, nil
}

func issueToken(userID, email, role, tokenType string, duration time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := &Claims{
		UserID: userID,
		Email:  email,
		Role:   role,
		Type:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			Issuer:    "sentinel-health-engine/user-service",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

// ParseAccessToken validates and parses an access token.
func ParseAccessToken(tokenString string) (*Claims, error) {
	return parseToken(tokenString, tokenTypeAccess)
}

// ParseRefreshToken validates and parses a refresh token.
func ParseRefreshToken(tokenString string) (*Claims, error) {
	return parseToken(tokenString, tokenTypeRefresh)
}

func parseToken(tokenString, expectedType string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	if claims.Type != expectedType {
		return nil, fmt.Errorf("wrong token type: expected %q got %q", expectedType, claims.Type)
	}

	return claims, nil
}
