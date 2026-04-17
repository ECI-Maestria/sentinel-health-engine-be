package user

import (
	"context"
	"fmt"

	appdevice "github.com/sentinel-health-engine/user-service/internal/application/device"
	appcart "github.com/sentinel-health-engine/user-service/internal/application/caretaker"
	domainuser "github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// DashboardPatient is the DTO returned per patient in the doctor dashboard.
type DashboardPatient struct {
	User           *domainuser.User
	DeviceCount    int
	CaretakerCount int
}

// GetDoctorDashboardUseCase returns all patients with device and caretaker counts.
type GetDoctorDashboardUseCase struct {
	users      domainuser.Repository
	listDevices    *appdevice.ListDevicesUseCase
	listCaretakers *appcart.ListCaretakersUseCase
}

// NewGetDoctorDashboardUseCase constructs the use case.
func NewGetDoctorDashboardUseCase(
	users domainuser.Repository,
	listDevices *appdevice.ListDevicesUseCase,
	listCaretakers *appcart.ListCaretakersUseCase,
) *GetDoctorDashboardUseCase {
	return &GetDoctorDashboardUseCase{
		users:          users,
		listDevices:    listDevices,
		listCaretakers: listCaretakers,
	}
}

// Execute lists all patients and enriches each with device and caretaker counts.
func (uc *GetDoctorDashboardUseCase) Execute(ctx context.Context) ([]*DashboardPatient, error) {
	patients, err := uc.users.ListByRole(ctx, domainuser.RolePatient)
	if err != nil {
		return nil, fmt.Errorf("list patients for dashboard: %w", err)
	}

	result := make([]*DashboardPatient, 0, len(patients))
	for _, p := range patients {
		devices, err := uc.listDevices.Execute(ctx, p.ID())
		if err != nil {
			devices = nil // degrade gracefully — count as 0
		}

		caretakers, err := uc.listCaretakers.Execute(ctx, p.ID())
		if err != nil {
			caretakers = nil // degrade gracefully — count as 0
		}

		result = append(result, &DashboardPatient{
			User:           p,
			DeviceCount:    len(devices),
			CaretakerCount: len(caretakers),
		})
	}

	return result, nil
}
