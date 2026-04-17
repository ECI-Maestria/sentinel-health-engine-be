package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// UserRepository implements user.Repository backed by PostgreSQL.
type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Save(ctx context.Context, u *user.User) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, first_name, last_name, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, u.ID(), u.Email(), u.PasswordHash(), string(u.Role()),
		u.FirstName(), u.LastName(), u.IsActive(), u.CreatedAt(), u.UpdatedAt())
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (r *UserRepository) Update(ctx context.Context, u *user.User) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE users SET
			email         = $2,
			password_hash = $3,
			first_name    = $4,
			last_name     = $5,
			is_active     = $6,
			updated_at    = $7
		WHERE id = $1
	`, u.ID(), u.Email(), u.PasswordHash(),
		u.FirstName(), u.LastName(), u.IsActive(), u.UpdatedAt())
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user %s not found", u.ID())
	}
	return nil
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*user.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, role, first_name, last_name, is_active, created_at, updated_at
		FROM users WHERE id = $1
	`, id)
	return scanUser(row)
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, role, first_name, last_name, is_active, created_at, updated_at
		FROM users WHERE email = $1
	`, email)
	return scanUser(row)
}

func (r *UserRepository) ListByRole(ctx context.Context, role user.Role) ([]*user.User, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, email, password_hash, role, first_name, last_name, is_active, created_at, updated_at
		FROM users WHERE role = $1 ORDER BY created_at DESC
	`, string(role))
	if err != nil {
		return nil, fmt.Errorf("list users by role: %w", err)
	}
	defer rows.Close()

	var users []*user.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE email = $1", email).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check email: %w", err)
	}
	return count > 0, nil
}

// scanUser reads a User from any pgx row-scanning interface.
func scanUser(row interface {
	Scan(dest ...any) error
}) (*user.User, error) {
	var (
		id           string
		email        string
		passwordHash string
		role         string
		firstName    string
		lastName     string
		isActive     bool
		createdAt    time.Time
		updatedAt    time.Time
	)

	if err := row.Scan(&id, &email, &passwordHash, &role, &firstName, &lastName, &isActive, &createdAt, &updatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("not found")
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}

	return user.Reconstitute(id, email, passwordHash, firstName, lastName, user.Role(role), isActive, createdAt, updatedAt), nil
}
