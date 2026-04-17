package auth

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// LoginCommand is the input DTO.
type LoginCommand struct {
	Email    string
	Password string
}

// LoginUseCase authenticates a user by email and password.
type LoginUseCase struct {
	users  user.Repository
	logger *zap.Logger
}

func NewLoginUseCase(users user.Repository, logger *zap.Logger) *LoginUseCase {
	return &LoginUseCase{users: users, logger: logger}
}

// Execute returns a JWT token pair on successful authentication.
func (uc *LoginUseCase) Execute(ctx context.Context, cmd LoginCommand) (*TokenPair, error) {
	u, err := uc.users.FindByEmail(ctx, cmd.Email)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	if !u.IsActive() {
		return nil, fmt.Errorf("account is inactive")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash()), []byte(cmd.Password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	pair, err := IssueTokenPair(u.ID(), u.Email(), string(u.Role()))
	if err != nil {
		return nil, fmt.Errorf("issue tokens: %w", err)
	}

	uc.logger.Info("user logged in",
		zap.String("userId", u.ID()),
		zap.String("role", string(u.Role())),
	)
	return pair, nil
}
