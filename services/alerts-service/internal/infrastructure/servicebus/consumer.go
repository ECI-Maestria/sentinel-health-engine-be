package servicebus

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"go.uber.org/zap"

	sharedevents "github.com/sentinel-health-engine/shared/events"
	"github.com/sentinel-health-engine/alerts-service/internal/application"
)

// AnomalyConsumer subscribes to "anomaly-detected" and dispatches to the use case.
type AnomalyConsumer struct {
	client           *azservicebus.Client
	topicName        string
	subscriptionName string
	useCase          *application.CreateAlertUseCase
	logger           *zap.Logger
}

func NewAnomalyConsumer(
	client *azservicebus.Client,
	topicName, subscriptionName string,
	useCase *application.CreateAlertUseCase,
	logger *zap.Logger,
) *AnomalyConsumer {
	return &AnomalyConsumer{
		client: client, topicName: topicName,
		subscriptionName: subscriptionName, useCase: useCase, logger: logger,
	}
}

func (c *AnomalyConsumer) Start(ctx context.Context) error {
	receiver, err := c.client.NewReceiverForSubscription(c.topicName, c.subscriptionName, nil)
	if err != nil {
		return err
	}
	defer receiver.Close(ctx) //nolint:errcheck

	c.logger.Info("alerts consumer started",
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

func (c *AnomalyConsumer) handle(ctx context.Context, recv *azservicebus.Receiver, msg *azservicebus.ReceivedMessage) {
	var event sharedevents.AnomalyDetectedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		c.logger.Error("unmarshal AnomalyDetectedEvent failed — dead-lettering", zap.Error(err))
		_ = recv.DeadLetterMessage(ctx, msg, nil)
		return
	}
	if err := c.useCase.Execute(ctx, event); err != nil {
		c.logger.Error("create alert failed", zap.Error(err), zap.String("patientId", event.PatientID))
		_ = recv.AbandonMessage(ctx, msg, nil)
		return
	}
	_ = recv.CompleteMessage(ctx, msg, nil)
}
