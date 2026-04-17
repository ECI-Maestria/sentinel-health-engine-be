package passwordreset

import "context"

// Repository is the port for PasswordResetToken persistence.
type Repository interface {
	Save(ctx context.Context, t *Token) error
	FindByCode(ctx context.Context, code string) (*Token, error)
	MarkUsed(ctx context.Context, code string) error
	// DeleteExpired removes codes that have expired or been used, for cleanup.
	DeleteExpired(ctx context.Context) error
}
