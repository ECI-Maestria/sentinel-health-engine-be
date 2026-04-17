// Package application contains the use cases for the Telemetry service.
// Use cases orchestrate domain objects and call ports — no infrastructure code here.
package application

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/sentinel-health-engine/telemetry-service/internal/domain"
)

// IngestTelemetryCommand is the input DTO for the ingest use case.
type IngestTelemetryCommand struct {
	DeviceID   string
	HeartRate  int
	SpO2       float64
	MeasuredAt time.Time
	RawPayload []byte // kept for audit
}

// EventPublisher is the port for publishing domain events to the message bus.
type EventPublisher interface {
	PublishTelemetryReceived(ctx context.Context, reading *domain.TelemetryReading) error
}

// IngestTelemetryUseCase handles ingestion of a single telemetry reading.
// Invariant enforced: "only authorized devices can submit telemetry."
type IngestTelemetryUseCase struct {
	repository     domain.TelemetryRepository
	deviceRegistry domain.DeviceRegistry
	publisher      EventPublisher
	logger         *zap.Logger
}

func NewIngestTelemetryUseCase(
	repo domain.TelemetryRepository,
	registry domain.DeviceRegistry,
	publisher EventPublisher,
	logger *zap.Logger,
) *IngestTelemetryUseCase {
	return &IngestTelemetryUseCase{
		repository:     repo,
		deviceRegistry: registry,
		publisher:      publisher,
		logger:         logger,
	}
}

// Execute processes one telemetry command.
func (uc *IngestTelemetryUseCase) Execute(ctx context.Context, cmd IngestTelemetryCommand) error {
	log := uc.logger.With(zap.String("deviceId", cmd.DeviceID))

	// 1. Authorize device (DDD invariant: reject unauthorized devices without persisting)
	patientID, authorized, err := uc.deviceRegistry.IsAuthorized(ctx, cmd.DeviceID)
	if err != nil {
		return fmt.Errorf("device registry lookup failed: %w", err)
	}
	if !authorized {
		log.Warn("telemetry rejected: device not authorized")
		return fmt.Errorf("device %s is not authorized", cmd.DeviceID)
	}

	// 2. Build and validate the aggregate (domain invariants enforced in constructor)
	reading, err := domain.NewTelemetryReading(
		cmd.DeviceID,
		patientID,
		cmd.HeartRate,
		cmd.SpO2,
		cmd.MeasuredAt,
	)
	if err != nil {
		log.Warn("telemetry rejected: invalid reading", zap.Error(err))
		return fmt.Errorf("invalid telemetry reading: %w", err)
	}

	// 3. Persist to Cosmos DB
	if err := uc.repository.Save(ctx, reading); err != nil {
		return fmt.Errorf("failed to persist telemetry reading: %w", err)
	}

	// 4. Publish domain event to Service Bus
	if err := uc.publisher.PublishTelemetryReceived(ctx, reading); err != nil {
		// Persistence succeeded. Log the error but don't fail the operation.
		// TODO: Implement outbox pattern for guaranteed delivery in production.
		log.Error("failed to publish TelemetryReceived event — reading persisted but event lost",
			zap.Error(err), zap.String("readingId", reading.ID()))
	}

	log.Info("telemetry ingested",
		zap.String("readingId", reading.ID()),
		zap.String("patientId", patientID),
		zap.Int("heartRate", cmd.HeartRate),
		zap.Float64("spO2", cmd.SpO2),
	)
	return nil
}
