package auth

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// ChangePasswordCommand is the input DTO.
type ChangePasswordCommand struct {
	UserID      string
	OldPassword string
	NewPassword string
}

// ChangePasswordUseCase lets an authenticated user change their own password.
type ChangePasswordUseCase struct {
	users  user.Repository
	logger *zap.Logger
}

func NewChangePasswordUseCase(users user.Repository, logger *zap.Logger) *ChangePasswordUseCase {
	return &ChangePasswordUseCase{users: users, logger: logger}
}

// Execute validates the current password and replaces it with the new one.
func (uc *ChangePasswordUseCase) Execute(ctx context.Context, cmd ChangePasswordCommand) error {
	if len(cmd.NewPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters")
	}

	u, err := uc.users.FindByID(ctx, cmd.UserID)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash()), []byte(cmd.OldPassword)); err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(cmd.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := u.ChangePassword(string(newHash)); err != nil {
		return err
	}

	if err := uc.users.Update(ctx, u); err != nil {
		return fmt.Errorf("persist password change: %w", err)
	}

	uc.logger.Info("password changed", zap.String("userId", u.ID()))
	return nil
}
