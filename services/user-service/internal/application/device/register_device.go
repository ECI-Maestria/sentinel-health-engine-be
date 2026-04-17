// Package device contains use cases for device management.
package device

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	domaindevice "github.com/sentinel-health-engine/user-service/internal/domain/device"
)

// IoTHubRegistry creates device identities in Azure IoT Hub.
type IoTHubRegistry interface {
	EnsureDevice(ctx context.Context, deviceID string) error
}

// RegisterDeviceCommand is the input DTO. Called by the mobile app on every login.
type RegisterDeviceCommand struct {
	UserID           string
	DeviceIdentifier string // IoT Hub device ID (e.g. "mobile-gateway-01")
	FCMToken         string
	Platform         string // "ANDROID" or "IOS"
	Name             string // optional human-readable label
}

// RegisterDeviceUseCase registers a new device or updates an existing one.
// Idempotent: if the device already exists for this user, it updates the FCM token.
type RegisterDeviceUseCase struct {
	devices   domaindevice.Repository
	iotHub    IoTHubRegistry // optional; nil = skip IoT Hub registration
	logger    *zap.Logger
}

func NewRegisterDeviceUseCase(devices domaindevice.Repository, iotHub IoTHubRegistry, logger *zap.Logger) *RegisterDeviceUseCase {
	return &RegisterDeviceUseCase{devices: devices, iotHub: iotHub, logger: logger}
}

// Execute upserts the device registration.
func (uc *RegisterDeviceUseCase) Execute(ctx context.Context, cmd RegisterDeviceCommand) (*domaindevice.Device, error) {
	platform := domaindevice.Platform(cmd.Platform)
	if !platform.IsValid() {
		return nil, fmt.Errorf("invalid platform %q: must be ANDROID or IOS", cmd.Platform)
	}

	existing, err := uc.devices.FindByUserIDAndIdentifier(ctx, cmd.UserID, cmd.DeviceIdentifier)
	if err == nil {
		// Device already registered — update FCM token.
		existing.UpdateFCMToken(cmd.FCMToken)
		if err := uc.devices.Update(ctx, existing); err != nil {
			return nil, fmt.Errorf("update device: %w", err)
		}
		uc.logger.Info("device updated",
			zap.String("deviceId", existing.ID()),
			zap.String("userId", cmd.UserID),
		)
		return existing, nil
	}

	// New device.
	name := cmd.Name
	if name == "" {
		name = cmd.DeviceIdentifier
	}

	d, err := domaindevice.NewDevice(uuid.NewString(), cmd.UserID, cmd.DeviceIdentifier, cmd.FCMToken, name, platform)
	if err != nil {
		return nil, fmt.Errorf("create device: %w", err)
	}

	if err := uc.devices.Save(ctx, d); err != nil {
		return nil, fmt.Errorf("persist device: %w", err)
	}

	// Register the device identity in IoT Hub so it can send telemetry.
	if uc.iotHub != nil {
		if err := uc.iotHub.EnsureDevice(ctx, cmd.DeviceIdentifier); err != nil {
			// Non-fatal: log and continue. The device is saved in DB; the admin
			// can create the IoT Hub identity manually if needed.
			uc.logger.Warn("failed to register device in IoT Hub",
				zap.String("identifier", cmd.DeviceIdentifier),
				zap.Error(err),
			)
		} else {
			uc.logger.Info("device registered in IoT Hub",
				zap.String("identifier", cmd.DeviceIdentifier),
			)
		}
	}

	uc.logger.Info("device registered",
		zap.String("deviceId", d.ID()),
		zap.String("userId", cmd.UserID),
		zap.String("identifier", cmd.DeviceIdentifier),
	)
	return d, nil
}
