package domain_test

import (
	"testing"
	"time"

	"github.com/sentinel-health-engine/telemetry-service/internal/domain"
)

// --- HeartRate ---

func TestNewHeartRate_Valid(t *testing.T) {
	cases := []int{0, 1, 60, 100, 299, 300}
	for _, bpm := range cases {
		hr, err := domain.NewHeartRate(bpm)
		if err != nil {
			t.Errorf("bpm=%d: unexpected error: %v", bpm, err)
		}
		if hr.Value() != bpm {
			t.Errorf("bpm=%d: got %d", bpm, hr.Value())
		}
	}
}

func TestNewHeartRate_Invalid(t *testing.T) {
	cases := []int{-1, -100, 301, 500}
	for _, bpm := range cases {
		_, err := domain.NewHeartRate(bpm)
		if err == nil {
			t.Errorf("bpm=%d: expected error, got nil", bpm)
		}
	}
}

// --- SpO2 ---

func TestNewSpO2_Valid(t *testing.T) {
	cases := []float64{0, 0.1, 50.5, 94.9, 99.9, 100}
	for _, pct := range cases {
		s, err := domain.NewSpO2(pct)
		if err != nil {
			t.Errorf("pct=%.1f: unexpected error: %v", pct, err)
		}
		if s.Value() != pct {
			t.Errorf("pct=%.1f: got %.1f", pct, s.Value())
		}
	}
}

func TestNewSpO2_Invalid(t *testing.T) {
	cases := []float64{-0.1, -50, 100.1, 200}
	for _, pct := range cases {
		_, err := domain.NewSpO2(pct)
		if err == nil {
			t.Errorf("pct=%.1f: expected error, got nil", pct)
		}
	}
}

// --- TelemetryReading ---

func TestNewTelemetryReading_Valid(t *testing.T) {
	now := time.Now()
	r, err := domain.NewTelemetryReading("device-01", "patient-uuid", 72, 98.5, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID() == "" {
		t.Error("ID should not be empty")
	}
	if r.DeviceID().String() != "device-01" {
		t.Errorf("DeviceID: got %q", r.DeviceID())
	}
	if r.PatientID().String() != "patient-uuid" {
		t.Errorf("PatientID: got %q", r.PatientID())
	}
	if r.HeartRate().Value() != 72 {
		t.Errorf("HeartRate: got %d", r.HeartRate().Value())
	}
	if r.SpO2().Value() != 98.5 {
		t.Errorf("SpO2: got %.1f", r.SpO2().Value())
	}
	if !r.MeasuredAt().Equal(now) {
		t.Errorf("MeasuredAt mismatch")
	}
	if r.ReceivedAt().IsZero() {
		t.Error("ReceivedAt should not be zero")
	}
}

func TestNewTelemetryReading_EmptyDeviceID(t *testing.T) {
	_, err := domain.NewTelemetryReading("", "patient-uuid", 72, 98.5, time.Now())
	if err == nil {
		t.Fatal("expected error for empty deviceID")
	}
}

func TestNewTelemetryReading_EmptyPatientID(t *testing.T) {
	_, err := domain.NewTelemetryReading("device-01", "", 72, 98.5, time.Now())
	if err == nil {
		t.Fatal("expected error for empty patientID")
	}
}

func TestNewTelemetryReading_InvalidHeartRate(t *testing.T) {
	_, err := domain.NewTelemetryReading("device-01", "patient-uuid", -1, 98.5, time.Now())
	if err == nil {
		t.Fatal("expected error for invalid heart rate")
	}
}

func TestNewTelemetryReading_InvalidSpO2(t *testing.T) {
	_, err := domain.NewTelemetryReading("device-01", "patient-uuid", 72, 101.0, time.Now())
	if err == nil {
		t.Fatal("expected error for invalid SpO2")
	}
}

func TestNewTelemetryReading_UniqueIDs(t *testing.T) {
	now := time.Now()
	r1, _ := domain.NewTelemetryReading("device-01", "patient-uuid", 72, 98.5, now)
	r2, _ := domain.NewTelemetryReading("device-01", "patient-uuid", 72, 98.5, now)
	if r1.ID() == r2.ID() {
		t.Error("consecutive readings should have different IDs")
	}
}
