package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/sentinel-health-engine/telemetry-service/internal/application"
	"github.com/sentinel-health-engine/telemetry-service/internal/domain"
)

// --- Mocks ---

type mockDeviceRegistry struct {
	patientID  string
	authorized bool
	err        error
}

func (m *mockDeviceRegistry) IsAuthorized(_ context.Context, _ string) (string, bool, error) {
	return m.patientID, m.authorized, m.err
}

type mockRepository struct {
	err error
}

func (m *mockRepository) Save(_ context.Context, _ *domain.TelemetryReading) error {
	return m.err
}

type mockPublisher struct {
	err     error
	called  bool
}

func (m *mockPublisher) PublishTelemetryReceived(_ context.Context, _ *domain.TelemetryReading) error {
	m.called = true
	return m.err
}

func newUseCase(registry *mockDeviceRegistry, repo *mockRepository, pub *mockPublisher) *application.IngestTelemetryUseCase {
	return application.NewIngestTelemetryUseCase(repo, registry, pub, zap.NewNop())
}

func validCmd() application.IngestTelemetryCommand {
	return application.IngestTelemetryCommand{
		DeviceID:   "device-01",
		HeartRate:  72,
		SpO2:       98.5,
		MeasuredAt: time.Now(),
	}
}

// --- Tests ---

func TestIngestTelemetry_Success(t *testing.T) {
	registry := &mockDeviceRegistry{patientID: "patient-uuid", authorized: true}
	repo := &mockRepository{}
	pub := &mockPublisher{}
	uc := newUseCase(registry, repo, pub)

	err := uc.Execute(context.Background(), validCmd())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pub.called {
		t.Error("expected PublishTelemetryReceived to be called")
	}
}

func TestIngestTelemetry_UnauthorizedDevice(t *testing.T) {
	registry := &mockDeviceRegistry{authorized: false}
	uc := newUseCase(registry, &mockRepository{}, &mockPublisher{})

	err := uc.Execute(context.Background(), validCmd())
	if err == nil {
		t.Fatal("expected error for unauthorized device")
	}
}

func TestIngestTelemetry_RegistryError(t *testing.T) {
	registry := &mockDeviceRegistry{err: errors.New("registry unavailable")}
	uc := newUseCase(registry, &mockRepository{}, &mockPublisher{})

	err := uc.Execute(context.Background(), validCmd())
	if err == nil {
		t.Fatal("expected error when registry fails")
	}
}

func TestIngestTelemetry_InvalidHeartRate(t *testing.T) {
	registry := &mockDeviceRegistry{patientID: "patient-uuid", authorized: true}
	uc := newUseCase(registry, &mockRepository{}, &mockPublisher{})

	cmd := validCmd()
	cmd.HeartRate = 999
	err := uc.Execute(context.Background(), cmd)
	if err == nil {
		t.Fatal("expected error for invalid heart rate")
	}
}

func TestIngestTelemetry_RepositoryError(t *testing.T) {
	registry := &mockDeviceRegistry{patientID: "patient-uuid", authorized: true}
	repo := &mockRepository{err: errors.New("cosmos unavailable")}
	pub := &mockPublisher{}
	uc := newUseCase(registry, repo, pub)

	err := uc.Execute(context.Background(), validCmd())
	if err == nil {
		t.Fatal("expected error when repository fails")
	}
}

func TestIngestTelemetry_PublisherError_DoesNotFailOperation(t *testing.T) {
	registry := &mockDeviceRegistry{patientID: "patient-uuid", authorized: true}
	repo := &mockRepository{}
	pub := &mockPublisher{err: errors.New("servicebus down")}
	uc := newUseCase(registry, repo, pub)

	err := uc.Execute(context.Background(), validCmd())
	if err != nil {
		t.Fatalf("publisher error should not fail the operation, got: %v", err)
	}
}
