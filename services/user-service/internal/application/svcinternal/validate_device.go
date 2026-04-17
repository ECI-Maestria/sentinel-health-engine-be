// Package internal contains use cases exposed only to other services via API key.
package svcinternal

import (
	"context"
	"fmt"

	domaindevice "github.com/sentinel-health-engine/user-service/internal/domain/device"
	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// DeviceValidationResult is returned to internal callers (telemetry-service).
type DeviceValidationResult struct {
	PatientID string
	UserID    string
	DeviceID  string
	IsActive  bool
}

// ValidateDeviceUseCase looks up a device by its IoT Hub identifier.
// Used by telemetry-service to resolve deviceId → patientId.
type ValidateDeviceUseCase struct {
	devices domaindevice.Repository
	users   user.Repository
}

func NewValidateDeviceUseCase(devices domaindevice.Repository, users user.Repository) *ValidateDeviceUseCase {
	return &ValidateDeviceUseCase{devices: devices, users: users}
}

// Execute returns patient info for a given IoT device identifier.
func (uc *ValidateDeviceUseCase) Execute(ctx context.Context, deviceIdentifier string) (*DeviceValidationResult, error) {
	d, err := uc.devices.FindByIdentifier(ctx, deviceIdentifier)
	if err != nil {
		return nil, fmt.Errorf("device %q not found", deviceIdentifier)
	}
	if !d.IsActive() {
		return nil, fmt.Errorf("device %q is inactive", deviceIdentifier)
	}

	owner, err := uc.users.FindByID(ctx, d.UserID())
	if err != nil {
		return nil, fmt.Errorf("device owner not found")
	}
	if owner.Role() != user.RolePatient {
		return nil, fmt.Errorf("device is not associated with a patient")
	}
	if !owner.IsActive() {
		return nil, fmt.Errorf("patient account is inactive")
	}

	return &DeviceValidationResult{
		PatientID: owner.ID(),
		UserID:    owner.ID(),
		DeviceID:  d.ID(),
		IsActive:  true,
	}, nil
}
