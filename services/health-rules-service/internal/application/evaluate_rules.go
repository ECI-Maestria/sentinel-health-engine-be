// Package application contains the use cases for the Health Rules bounded context.
package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	sharedevents "github.com/sentinel-health-engine/shared/events"
	"github.com/sentinel-health-engine/health-rules-service/internal/domain"
)

// AnomalyPublisher is the port for publishing anomaly events to the message bus.
type AnomalyPublisher interface {
	PublishAnomalyDetected(ctx context.Context, event sharedevents.AnomalyDetectedEvent) error
}

// EvaluateRulesUseCase processes a TelemetryReceivedEvent and checks health rules.
type EvaluateRulesUseCase struct {
	publisher AnomalyPublisher
	logger    *zap.Logger
}

func NewEvaluateRulesUseCase(publisher AnomalyPublisher, logger *zap.Logger) *EvaluateRulesUseCase {
	return &EvaluateRulesUseCase{publisher: publisher, logger: logger}
}

// Execute evaluates all active health rules against a telemetry event.
func (uc *EvaluateRulesUseCase) Execute(ctx context.Context, event sharedevents.TelemetryReceivedEvent) error {
	log := uc.logger.With(
		zap.String("readingId", event.ReadingID),
		zap.String("patientId", event.PatientID),
		zap.Int("heartRate", event.HeartRate),
		zap.Float64("spO2", event.SpO2),
	)

	rules := domain.DefaultRules()

	result := domain.EvaluateRules(rules, domain.EvaluationInput{
		PatientID: event.PatientID,
		DeviceID:  event.DeviceID,
		ReadingID: event.ReadingID,
		HeartRate: float64(event.HeartRate),
		SpO2:      event.SpO2,
	})

	if !result.HasAnomalies {
		log.Debug("no anomalies detected")
		return nil
	}

	log.Warn("anomaly detected",
		zap.Int("violations", len(result.Violations)),
		zap.String("maxSeverity", string(result.MaxSeverity)),
	)

	anomalyEvent := sharedevents.AnomalyDetectedEvent{
		EventID:     uuid.New().String(),
		EventType:   sharedevents.EventTypeAnomalyDetected,
		OccurredAt:  time.Now().UTC(),
		ReadingID:   event.ReadingID,
		PatientID:   event.PatientID,
		DeviceID:    event.DeviceID,
		HeartRate:   event.HeartRate,
		SpO2:        event.SpO2,
		Violations:  result.Violations,
		MaxSeverity: result.MaxSeverity,
		Timestamp:   event.Timestamp,
	}

	if err := uc.publisher.PublishAnomalyDetected(ctx, anomalyEvent); err != nil {
		return fmt.Errorf("publish AnomalyDetected: %w", err)
	}
	return nil
}
