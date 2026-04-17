// Package passwordreset contains the PasswordResetToken aggregate.
package passwordreset

import (
	"fmt"
	"time"
)

const codeTTL = time.Minute

// Token represents a one-time password reset OTP code.
type Token struct {
	code      string
	userID    string
	expiresAt time.Time
	used      bool
	createdAt time.Time
}

// NewCode creates a new unused reset OTP code.
func NewCode(code, userID string) (*Token, error) {
	if code == "" {
		return nil, fmt.Errorf("code is required")
	}
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	now := time.Now().UTC()
	return &Token{
		code:      code,
		userID:    userID,
		expiresAt: now.Add(codeTTL),
		used:      false,
		createdAt: now,
	}, nil
}

// Reconstitute rebuilds a Token from persisted data.
func Reconstitute(code, userID string, expiresAt time.Time, used bool, createdAt time.Time) *Token {
	return &Token{
		code:      code,
		userID:    userID,
		expiresAt: expiresAt,
		used:      used,
		createdAt: createdAt,
	}
}

func (t *Token) Code() string         { return t.code }
func (t *Token) UserID() string       { return t.userID }
func (t *Token) ExpiresAt() time.Time { return t.expiresAt }
func (t *Token) Used() bool           { return t.used }
func (t *Token) CreatedAt() time.Time { return t.createdAt }

// IsValid returns true if the code has not been used and has not expired.
func (t *Token) IsValid() bool {
	return !t.used && time.Now().UTC().Before(t.expiresAt)
}

// MarkUsed invalidates the code so it cannot be reused.
func (t *Token) MarkUsed() {
	t.used = true
}
