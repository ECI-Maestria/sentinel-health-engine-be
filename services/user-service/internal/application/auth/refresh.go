package auth

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// RefreshUseCase issues a new token pair from a valid refresh token.
type RefreshUseCase struct {
	users  user.Repository
	logger *zap.Logger
}

func NewRefreshUseCase(users user.Repository, logger *zap.Logger) *RefreshUseCase {
	return &RefreshUseCase{users: users, logger: logger}
}

// Execute validates the refresh token and returns a new token pair.
func (uc *RefreshUseCase) Execute(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	u, err := uc.users.FindByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	if !u.IsActive() {
		return nil, fmt.Errorf("account is inactive")
	}

	pair, err := IssueTokenPair(u.ID(), u.Email(), string(u.Role()))
	if err != nil {
		return nil, fmt.Errorf("issue tokens: %w", err)
	}

	uc.logger.Info("token refreshed", zap.String("userId", u.ID()))
	return pair, nil
}
