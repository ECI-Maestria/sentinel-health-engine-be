package servicebus

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"go.uber.org/zap"

	sharedevents "github.com/sentinel-health-engine/shared/events"
)

// AnomalyPublisher publishes AnomalyDetectedEvent to the "anomaly-detected" topic.
type AnomalyPublisher struct {
	client    *azservicebus.Client
	topicName string
	logger    *zap.Logger
}

func NewAnomalyPublisher(client *azservicebus.Client, topicName string, logger *zap.Logger) *AnomalyPublisher {
	return &AnomalyPublisher{client: client, topicName: topicName, logger: logger}
}

func (p *AnomalyPublisher) PublishAnomalyDetected(ctx context.Context, event sharedevents.AnomalyDetectedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal AnomalyDetectedEvent: %w", err)
	}

	sender, err := p.client.NewSender(p.topicName, nil)
	if err != nil {
		return fmt.Errorf("create sender: %w", err)
	}
	defer sender.Close(ctx) //nolint:errcheck

	contentType := "application/json"
	msg := &azservicebus.Message{
		Body:        body,
		ContentType: &contentType,
		ApplicationProperties: map[string]interface{}{
			"eventType": sharedevents.EventTypeAnomalyDetected,
			"patientId": event.PatientID,
			"severity":  string(event.MaxSeverity),
		},
	}

	if err := sender.SendMessage(ctx, msg, nil); err != nil {
		return fmt.Errorf("send to topic %q: %w", p.topicName, err)
	}

	p.logger.Info("published AnomalyDetected",
		zap.String("patientId", event.PatientID),
		zap.String("severity", string(event.MaxSeverity)),
	)
	return nil
}
