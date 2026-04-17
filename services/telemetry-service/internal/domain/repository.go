package domain

import "context"

// TelemetryRepository is the port (interface) for persisting telemetry readings.
// The implementation lives in the infrastructure layer (Cosmos DB).
type TelemetryRepository interface {
	Save(ctx context.Context, reading *TelemetryReading) error
}

// DeviceRegistry is the port for checking device authorization.
// For the PoC this is implemented in-memory. In production, it queries
// the Registration bounded context via HTTP/gRPC.
type DeviceRegistry interface {
	IsAuthorized(ctx context.Context, deviceID string) (patientID string, authorized bool, err error)
}
