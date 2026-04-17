// Package servicebus implements the EventPublisher port using Azure Service Bus.
package servicebus

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/google/uuid"
	"go.uber.org/zap"

	sharedevents "github.com/sentinel-health-engine/shared/events"
	"github.com/sentinel-health-engine/telemetry-service/internal/domain"
)

// TelemetryEventPublisher publishes domain events to Azure Service Bus topics.
type TelemetryEventPublisher struct {
	client    *azservicebus.Client
	topicName string
	logger    *zap.Logger
}

func NewTelemetryEventPublisher(client *azservicebus.Client, topicName string, logger *zap.Logger) *TelemetryEventPublisher {
	return &TelemetryEventPublisher{client: client, topicName: topicName, logger: logger}
}

// PublishTelemetryReceived sends TelemetryReceivedEvent to the Service Bus topic.
func (p *TelemetryEventPublisher) PublishTelemetryReceived(ctx context.Context, reading *domain.TelemetryReading) error {
	event := sharedevents.TelemetryReceivedEvent{
		EventID:    uuid.New().String(),
		EventType:  sharedevents.EventTypeTelemetryReceived,
		OccurredAt: time.Now().UTC(),
		ReadingID:  reading.ID(),
		DeviceID:   reading.DeviceID().String(),
		PatientID:  reading.PatientID().String(),
		HeartRate:  reading.HeartRate().Value(),
		SpO2:       reading.SpO2().Value(),
		Timestamp:  reading.MeasuredAt(),
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal TelemetryReceivedEvent: %w", err)
	}

	sender, err := p.client.NewSender(p.topicName, nil)
	if err != nil {
		return fmt.Errorf("create sender for topic %q: %w", p.topicName, err)
	}
	defer sender.Close(ctx) //nolint:errcheck

	contentType := "application/json"
	msg := &azservicebus.Message{
		Body:        body,
		ContentType: &contentType,
		ApplicationProperties: map[string]interface{}{
			"eventType": sharedevents.EventTypeTelemetryReceived,
			"patientId": reading.PatientID().String(),
		},
	}

	if err := sender.SendMessage(ctx, msg, nil); err != nil {
		return fmt.Errorf("send to Service Bus topic %q: %w", p.topicName, err)
	}

	p.logger.Debug("published TelemetryReceived",
		zap.String("readingId", reading.ID()),
		zap.String("topic", p.topicName),
	)
	return nil
}
