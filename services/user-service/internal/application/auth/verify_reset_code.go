package auth

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/sentinel-health-engine/user-service/internal/domain/passwordreset"
	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// VerifyResetCodeCommand is the input DTO.
type VerifyResetCodeCommand struct {
	Code string
}

// VerifyResetCodeResult is returned on success.
// Email is masked (e.g. "d***@hospital.com") so the UI can confirm
// ownership without exposing the full address.
type VerifyResetCodeResult struct {
	MaskedEmail string
}

// VerifyResetCodeUseCase checks that an OTP code is valid without consuming it.
// The code remains usable for the subsequent reset-password call.
type VerifyResetCodeUseCase struct {
	users  user.Repository
	tokens passwordreset.Repository
	logger *zap.Logger
}

func NewVerifyResetCodeUseCase(users user.Repository, tokens passwordreset.Repository, logger *zap.Logger) *VerifyResetCodeUseCase {
	return &VerifyResetCodeUseCase{users: users, tokens: tokens, logger: logger}
}

// Execute validates the OTP code and returns a masked email for UI confirmation.
// It does NOT mark the code as used so it can still be submitted in reset-password.
func (uc *VerifyResetCodeUseCase) Execute(ctx context.Context, cmd VerifyResetCodeCommand) (*VerifyResetCodeResult, error) {
	t, err := uc.tokens.FindByCode(ctx, cmd.Code)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired reset code")
	}
	if !t.IsValid() {
		return nil, fmt.Errorf("invalid or expired reset code")
	}

	u, err := uc.users.FindByID(ctx, t.UserID())
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	uc.logger.Info("reset code verified", zap.String("userId", u.ID()))
	return &VerifyResetCodeResult{MaskedEmail: maskEmail(u.Email())}, nil
}

// maskEmail turns "doctor@hospital.com" into "d***@hospital.com".
func maskEmail(email string) string {
	for i, ch := range email {
		if ch == '@' {
			if i <= 1 {
				return email
			}
			return string([]rune(email)[:1]) + "***" + email[i:]
		}
	}
	return email
}
