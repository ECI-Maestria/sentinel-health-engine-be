package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sentinel-health-engine/user-service/internal/domain/passwordreset"
)

// PasswordResetRepository implements passwordreset.Repository backed by PostgreSQL.
type PasswordResetRepository struct {
	pool *pgxpool.Pool
}

func NewPasswordResetRepository(pool *pgxpool.Pool) *PasswordResetRepository {
	return &PasswordResetRepository{pool: pool}
}

func (r *PasswordResetRepository) Save(ctx context.Context, t *passwordreset.Token) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO password_reset_tokens (code, user_id, expires_at, used, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, t.Code(), t.UserID(), t.ExpiresAt(), t.Used(), t.CreatedAt())
	if err != nil {
		return fmt.Errorf("insert password reset code: %w", err)
	}
	return nil
}

func (r *PasswordResetRepository) FindByCode(ctx context.Context, code string) (*passwordreset.Token, error) {
	var (
		cod       string
		userID    string
		expiresAt time.Time
		used      bool
		createdAt time.Time
	)
	err := r.pool.QueryRow(ctx, `
		SELECT code, user_id, expires_at, used, created_at
		FROM password_reset_tokens WHERE code = $1
	`, code).Scan(&cod, &userID, &expiresAt, &used, &createdAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("code not found")
		}
		return nil, fmt.Errorf("find code: %w", err)
	}
	return passwordreset.Reconstitute(cod, userID, expiresAt, used, createdAt), nil
}

func (r *PasswordResetRepository) MarkUsed(ctx context.Context, code string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE password_reset_tokens SET used = true WHERE code = $1
	`, code)
	if err != nil {
		return fmt.Errorf("mark code used: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("code not found")
	}
	return nil
}

func (r *PasswordResetRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		DELETE FROM password_reset_tokens WHERE expires_at < NOW() OR used = true
	`)
	if err != nil {
		return fmt.Errorf("delete expired codes: %w", err)
	}
	return nil
}
