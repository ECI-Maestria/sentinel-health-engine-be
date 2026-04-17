// Package domain contains the core business logic for the Telemetry bounded context.
// This package has ZERO infrastructure dependencies — no Azure SDKs, no HTTP frameworks.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// --- Value Objects ---

// HeartRate represents beats per minute. Must be physiologically plausible.
type HeartRate int

func NewHeartRate(bpm int) (HeartRate, error) {
	if bpm < 0 || bpm > 300 {
		return 0, errors.New("heart rate must be between 0 and 300 bpm")
	}
	return HeartRate(bpm), nil
}

func (h HeartRate) Value() int { return int(h) }

// SpO2 represents blood oxygen saturation as a percentage (0-100).
type SpO2 float64

func NewSpO2(pct float64) (SpO2, error) {
	if pct < 0 || pct > 100 {
		return 0, errors.New("SpO2 must be between 0 and 100 percent")
	}
	return SpO2(pct), nil
}

func (s SpO2) Value() float64 { return float64(s) }

// DeviceID is the unique identifier of a registered wearable device.
type DeviceID string

func (d DeviceID) String() string { return string(d) }

// PatientID is the unique identifier of a patient.
type PatientID string

func (p PatientID) String() string { return string(p) }

// --- Aggregate Root ---

// TelemetryReading is the aggregate root for the Telemetry bounded context.
// Represents a single biometric measurement from a wearable device.
type TelemetryReading struct {
	id         string
	deviceID   DeviceID
	patientID  PatientID
	heartRate  HeartRate
	spO2       SpO2
	measuredAt time.Time
	receivedAt time.Time
}

// NewTelemetryReading creates a validated TelemetryReading aggregate.
// This is the only constructor — all domain invariants are enforced here.
func NewTelemetryReading(
	deviceID string,
	patientID string,
	heartRateBPM int,
	spO2Pct float64,
	measuredAt time.Time,
) (*TelemetryReading, error) {
	if deviceID == "" {
		return nil, errors.New("deviceID cannot be empty")
	}
	if patientID == "" {
		return nil, errors.New("patientID cannot be empty")
	}

	hr, err := NewHeartRate(heartRateBPM)
	if err != nil {
		return nil, err
	}

	spo2, err := NewSpO2(spO2Pct)
	if err != nil {
		return nil, err
	}

	return &TelemetryReading{
		id:         uuid.New().String(),
		deviceID:   DeviceID(deviceID),
		patientID:  PatientID(patientID),
		heartRate:  hr,
		spO2:       spo2,
		measuredAt: measuredAt,
		receivedAt: time.Now().UTC(),
	}, nil
}

// Read-only accessors (private fields enforced via value objects)
func (t *TelemetryReading) ID() string            { return t.id }
func (t *TelemetryReading) DeviceID() DeviceID    { return t.deviceID }
func (t *TelemetryReading) PatientID() PatientID  { return t.patientID }
func (t *TelemetryReading) HeartRate() HeartRate  { return t.heartRate }
func (t *TelemetryReading) SpO2() SpO2            { return t.spO2 }
func (t *TelemetryReading) MeasuredAt() time.Time { return t.measuredAt }
func (t *TelemetryReading) ReceivedAt() time.Time { return t.receivedAt }
