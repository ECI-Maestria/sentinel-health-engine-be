// Package device contains the Device aggregate.
// A Device is the smartphone used by a patient that acts simultaneously as:
//   - IoT gateway (receives smartwatch data via Bluetooth and sends to IoT Hub)
//   - Push notification target (identified by FCM token)
package device

import (
	"fmt"
	"time"
)

// Platform identifies the mobile operating system.
type Platform string

const (
	PlatformAndroid Platform = "ANDROID"
	PlatformIOS     Platform = "IOS"
)

// IsValid returns true if the platform is a known value.
func (p Platform) IsValid() bool {
	return p == PlatformAndroid || p == PlatformIOS
}

// Device represents a patient's smartphone registered in the system.
type Device struct {
	id               string
	userID           string
	deviceIdentifier string // IoT Hub device ID (e.g. "mobile-gateway-01")
	fcmToken         string // Firebase Cloud Messaging token for push notifications
	platform         Platform
	name             string
	isActive         bool
	lastSeenAt       *time.Time
	createdAt        time.Time
	updatedAt        time.Time
}

// NewDevice creates a Device enforcing domain invariants.
func NewDevice(id, userID, deviceIdentifier, fcmToken, name string, platform Platform) (*Device, error) {
	if id == "" {
		return nil, fmt.Errorf("device id is required")
	}
	if userID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if deviceIdentifier == "" {
		return nil, fmt.Errorf("device identifier is required")
	}
	if !platform.IsValid() {
		return nil, fmt.Errorf("invalid platform %q", platform)
	}

	now := time.Now().UTC()
	return &Device{
		id:               id,
		userID:           userID,
		deviceIdentifier: deviceIdentifier,
		fcmToken:         fcmToken,
		platform:         platform,
		name:             name,
		isActive:         true,
		createdAt:        now,
		updatedAt:        now,
	}, nil
}

// Reconstitute rebuilds a Device from persisted data.
func Reconstitute(id, userID, deviceIdentifier, fcmToken, name string, platform Platform, isActive bool, lastSeenAt *time.Time, createdAt, updatedAt time.Time) *Device {
	return &Device{
		id:               id,
		userID:           userID,
		deviceIdentifier: deviceIdentifier,
		fcmToken:         fcmToken,
		platform:         platform,
		name:             name,
		isActive:         isActive,
		lastSeenAt:       lastSeenAt,
		createdAt:        createdAt,
		updatedAt:        updatedAt,
	}
}

// Read-only accessors.
func (d *Device) ID() string               { return d.id }
func (d *Device) UserID() string           { return d.userID }
func (d *Device) DeviceIdentifier() string { return d.deviceIdentifier }
func (d *Device) FCMToken() string         { return d.fcmToken }
func (d *Device) Platform() Platform       { return d.platform }
func (d *Device) Name() string             { return d.name }
func (d *Device) IsActive() bool           { return d.isActive }
func (d *Device) LastSeenAt() *time.Time   { return d.lastSeenAt }
func (d *Device) CreatedAt() time.Time     { return d.createdAt }
func (d *Device) UpdatedAt() time.Time     { return d.updatedAt }

// UpdateFCMToken replaces the FCM token and records activity.
func (d *Device) UpdateFCMToken(token string) {
	d.fcmToken = token
	now := time.Now().UTC()
	d.lastSeenAt = &now
	d.updatedAt = now
}

// RecordSeen records the last time this device was active.
func (d *Device) RecordSeen() {
	now := time.Now().UTC()
	d.lastSeenAt = &now
	d.updatedAt = now
}

// Deactivate marks the device as inactive.
func (d *Device) Deactivate() {
	d.isActive = false
	d.updatedAt = time.Now().UTC()
}
