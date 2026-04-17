package doctor

import (
	"context"
	"fmt"

	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// ListDoctorsUseCase returns all active doctors in the system.
type ListDoctorsUseCase struct {
	users user.Repository
}

func NewListDoctorsUseCase(users user.Repository) *ListDoctorsUseCase {
	return &ListDoctorsUseCase{users: users}
}

func (uc *ListDoctorsUseCase) Execute(ctx context.Context) ([]*user.User, error) {
	doctors, err := uc.users.ListByRole(ctx, user.RoleDoctor)
	if err != nil {
		return nil, fmt.Errorf("list doctors: %w", err)
	}
	return doctors, nil
}
