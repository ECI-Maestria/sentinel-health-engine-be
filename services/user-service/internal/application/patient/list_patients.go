package patient

import (
	"context"
	"fmt"

	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// ListPatientsUseCase returns all patients (Doctor-only).
type ListPatientsUseCase struct {
	users user.Repository
}

func NewListPatientsUseCase(users user.Repository) *ListPatientsUseCase {
	return &ListPatientsUseCase{users: users}
}

func (uc *ListPatientsUseCase) Execute(ctx context.Context) ([]*user.User, error) {
	patients, err := uc.users.ListByRole(ctx, user.RolePatient)
	if err != nil {
		return nil, fmt.Errorf("list patients: %w", err)
	}
	return patients, nil
}
