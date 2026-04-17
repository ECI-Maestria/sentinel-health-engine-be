// Package events defines the domain events shared across all microservices.
// These structs are the async communication contracts via Azure Service Bus.
package events

import "time"

const (
	// EventTypeTelemetryReceived is published by Telemetry Service after a valid reading is ingested.
	EventTypeTelemetryReceived = "telemetry.received"
	// EventTypeTelemetryRejected is published when a reading is rejected (device not authorized).
	EventTypeTelemetryRejected = "telemetry.rejected"
)

// TelemetryReceivedEvent is emitted after a telemetry reading passes validation and is persisted.
// Health Rules Service subscribes to this event.
type TelemetryReceivedEvent struct {
	EventID    string    `json:"eventId"`
	EventType  string    `json:"eventType"`
	OccurredAt time.Time `json:"occurredAt"`
	ReadingID  string    `json:"readingId"`
	DeviceID   string    `json:"deviceId"`
	PatientID  string    `json:"patientId"`
	HeartRate  int       `json:"heartRate"` // bpm
	SpO2       float64   `json:"spO2"`      // percentage (0-100)
	Timestamp  time.Time `json:"timestamp"` // time of measurement on device
}

// TelemetryRejectedEvent is emitted when a reading cannot be processed.
type TelemetryRejectedEvent struct {
	EventID    string    `json:"eventId"`
	EventType  string    `json:"eventType"`
	OccurredAt time.Time `json:"occurredAt"`
	DeviceID   string    `json:"deviceId"`
	Reason     string    `json:"reason"`
}
