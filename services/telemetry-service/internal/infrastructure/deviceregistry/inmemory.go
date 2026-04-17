// Package deviceregistry provides an in-memory DeviceRegistry implementation.
// For the PoC, authorized device→patient mappings are loaded from AUTHORIZED_DEVICES env var.
// In production, this will query the Registration bounded context via HTTP/gRPC.
package deviceregistry

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// InMemoryDeviceRegistry implements domain.DeviceRegistry.
// AUTHORIZED_DEVICES format: "deviceId1:patientId1,deviceId2:patientId2"
type InMemoryDeviceRegistry struct {
	mappings map[string]string // deviceID → patientID
}

func NewInMemoryDeviceRegistry() (*InMemoryDeviceRegistry, error) {
	raw := os.Getenv("AUTHORIZED_DEVICES")
	if raw == "" {
		return nil, fmt.Errorf("AUTHORIZED_DEVICES env var is required (format: deviceId:patientId,...)")
	}

	mappings := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid device mapping %q — expected deviceId:patientId", pair)
		}
		mappings[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	return &InMemoryDeviceRegistry{mappings: mappings}, nil
}

func (r *InMemoryDeviceRegistry) IsAuthorized(_ context.Context, deviceID string) (string, bool, error) {
	patientID, ok := r.mappings[deviceID]
	return patientID, ok, nil
}
