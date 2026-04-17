// Package user contains use cases for user profile management.
package user

import (
	"context"
	"fmt"

	domainuser "github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// GetMeUseCase returns the profile of the authenticated user.
type GetMeUseCase struct {
	users domainuser.Repository
}

func NewGetMeUseCase(users domainuser.Repository) *GetMeUseCase {
	return &GetMeUseCase{users: users}
}

func (uc *GetMeUseCase) Execute(ctx context.Context, userID string) (*domainuser.User, error) {
	u, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}
	return u, nil
}
