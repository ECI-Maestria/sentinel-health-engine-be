package device

import (
	"context"
	"fmt"

	domaindevice "github.com/sentinel-health-engine/user-service/internal/domain/device"
)

// ListDevicesUseCase returns all devices belonging to a user.
type ListDevicesUseCase struct {
	devices domaindevice.Repository
}

func NewListDevicesUseCase(devices domaindevice.Repository) *ListDevicesUseCase {
	return &ListDevicesUseCase{devices: devices}
}

func (uc *ListDevicesUseCase) Execute(ctx context.Context, userID string) ([]*domaindevice.Device, error) {
	devs, err := uc.devices.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	return devs, nil
}
