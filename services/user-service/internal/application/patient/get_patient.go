package patient

import (
	"context"
	"fmt"

	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// GetPatientUseCase fetches a single patient by ID.
type GetPatientUseCase struct {
	users user.Repository
}

func NewGetPatientUseCase(users user.Repository) *GetPatientUseCase {
	return &GetPatientUseCase{users: users}
}

func (uc *GetPatientUseCase) Execute(ctx context.Context, patientID string) (*user.User, error) {
	u, err := uc.users.FindByID(ctx, patientID)
	if err != nil {
		return nil, fmt.Errorf("patient not found")
	}
	if u.Role() != user.RolePatient {
		return nil, fmt.Errorf("patient not found")
	}
	return u, nil
}
