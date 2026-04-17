// Package servicebus contains consumer and publisher for health-rules-service.
package servicebus

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"go.uber.org/zap"

	sharedevents "github.com/sentinel-health-engine/shared/events"
	"github.com/sentinel-health-engine/health-rules-service/internal/application"
)

// TelemetryConsumer subscribes to "telemetry-received" and dispatches to the use case.
type TelemetryConsumer struct {
	client           *azservicebus.Client
	topicName        string
	subscriptionName string
	useCase          *application.EvaluateRulesUseCase
	logger           *zap.Logger
}

func NewTelemetryConsumer(
	client *azservicebus.Client,
	topicName, subscriptionName string,
	useCase *application.EvaluateRulesUseCase,
	logger *zap.Logger,
) *TelemetryConsumer {
	return &TelemetryConsumer{
		client: client, topicName: topicName,
		subscriptionName: subscriptionName, useCase: useCase, logger: logger,
	}
}

// Start blocks consuming until ctx is cancelled.
func (c *TelemetryConsumer) Start(ctx context.Context) error {
	receiver, err := c.client.NewReceiverForSubscription(c.topicName, c.subscriptionName, nil)
	if err != nil {
		return err
	}
	defer receiver.Close(ctx) //nolint:errcheck

	c.logger.Info("health-rules consumer started",
		zap.String("topic", c.topicName),
		zap.String("subscription", c.subscriptionName),
	)

	for {
		messages, err := receiver.ReceiveMessages(ctx, 10, nil)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Error("receive error", zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}

		for _, msg := range messages {
			c.handle(ctx, receiver, msg)
		}
	}
}

func (c *TelemetryConsumer) handle(ctx context.Context, recv *azservicebus.Receiver, msg *azservicebus.ReceivedMessage) {
	var event sharedevents.TelemetryReceivedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		c.logger.Error("unmarshal TelemetryReceivedEvent failed — dead-lettering", zap.Error(err))
		_ = recv.DeadLetterMessage(ctx, msg, nil)
		return
	}

	if err := c.useCase.Execute(ctx, event); err != nil {
		c.logger.Error("evaluate rules failed", zap.Error(err), zap.String("readingId", event.ReadingID))
		_ = recv.AbandonMessage(ctx, msg, nil) // retry
		return
	}
	_ = recv.CompleteMessage(ctx, msg, nil)
}
