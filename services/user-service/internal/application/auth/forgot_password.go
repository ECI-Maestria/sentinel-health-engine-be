package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"go.uber.org/zap"

	"github.com/sentinel-health-engine/user-service/internal/domain/passwordreset"
	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// PasswordResetEmailSender is the port for sending reset emails.
type PasswordResetEmailSender interface {
	SendPasswordResetCode(ctx context.Context, email, fullName, code string) error
}

// RequestPasswordResetCommand is the input DTO.
type RequestPasswordResetCommand struct {
	Email string
}

// RequestPasswordResetUseCase generates a 6-digit OTP code and sends it via email.
type RequestPasswordResetUseCase struct {
	users       user.Repository
	tokens      passwordreset.Repository
	emailSender PasswordResetEmailSender
	logger      *zap.Logger
}

func NewRequestPasswordResetUseCase(
	users user.Repository,
	tokens passwordreset.Repository,
	emailSender PasswordResetEmailSender,
	logger *zap.Logger,
) *RequestPasswordResetUseCase {
	return &RequestPasswordResetUseCase{users: users, tokens: tokens, emailSender: emailSender, logger: logger}
}

// Execute always returns nil to prevent email enumeration attacks.
// If the email exists, a 6-digit OTP code valid for 1 minute is sent.
func (uc *RequestPasswordResetUseCase) Execute(ctx context.Context, cmd RequestPasswordResetCommand) error {
	u, err := uc.users.FindByEmail(ctx, cmd.Email)
	if err != nil {
		// Don't reveal whether the email exists.
		uc.logger.Info("password reset requested for unknown email", zap.String("email", cmd.Email))
		return nil
	}
	if !u.IsActive() {
		return nil
	}

	rawCode, err := generateOTPCode()
	if err != nil {
		return fmt.Errorf("generate otp code: %w", err)
	}

	token, err := passwordreset.NewCode(rawCode, u.ID())
	if err != nil {
		return fmt.Errorf("create reset code: %w", err)
	}

	if err := uc.tokens.Save(ctx, token); err != nil {
		return fmt.Errorf("persist reset code: %w", err)
	}

	if err := uc.emailSender.SendPasswordResetCode(ctx, u.Email(), u.FullName(), rawCode); err != nil {
		uc.logger.Error("failed to send password reset code",
			zap.String("userId", u.ID()), zap.Error(err))
		// Don't fail the request — code is saved, user can retry.
	}

	uc.logger.Info("password reset code issued", zap.String("userId", u.ID()))
	return nil
}

// generateOTPCode produces a cryptographically random 6-digit numeric code (000000–999999).
func generateOTPCode() (string, error) {
	max := big.NewInt(1_000_000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
