package auth

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/sentinel-health-engine/user-service/internal/domain/passwordreset"
	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// ResetPasswordCommand is the input DTO.
type ResetPasswordCommand struct {
	Code        string
	NewPassword string
}

// ResetPasswordUseCase validates the OTP code and sets a new password.
type ResetPasswordUseCase struct {
	users  user.Repository
	tokens passwordreset.Repository
	logger *zap.Logger
}

func NewResetPasswordUseCase(users user.Repository, tokens passwordreset.Repository, logger *zap.Logger) *ResetPasswordUseCase {
	return &ResetPasswordUseCase{users: users, tokens: tokens, logger: logger}
}

// Execute validates the OTP code and replaces the user's password.
func (uc *ResetPasswordUseCase) Execute(ctx context.Context, cmd ResetPasswordCommand) error {
	if len(cmd.NewPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters")
	}

	t, err := uc.tokens.FindByCode(ctx, cmd.Code)
	if err != nil {
		return fmt.Errorf("invalid or expired reset code")
	}
	if !t.IsValid() {
		return fmt.Errorf("invalid or expired reset code")
	}

	u, err := uc.users.FindByID(ctx, t.UserID())
	if err != nil {
		return fmt.Errorf("user not found")
	}
	if !u.IsActive() {
		return fmt.Errorf("account is inactive")
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(cmd.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := u.ChangePassword(string(newHash)); err != nil {
		return err
	}
	if err := uc.users.Update(ctx, u); err != nil {
		return fmt.Errorf("persist password: %w", err)
	}

	if err := uc.tokens.MarkUsed(ctx, cmd.Code); err != nil {
		uc.logger.Warn("failed to mark reset code as used", zap.Error(err))
	}

	uc.logger.Info("password reset successful", zap.String("userId", u.ID()))
	return nil
}
