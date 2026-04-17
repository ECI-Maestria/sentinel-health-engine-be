// Package iothub contains the Azure IoT Hub consumer adapter.
// Reads from the IoT Hub built-in Event Hub compatible endpoint.
//
// NOTE: We use ConsumerClient + NewPartitionClient directly instead of
// azeventhubs.Processor. The Processor pattern uses epoch (exclusive) receivers
// which conflict with IoT Hub's built-in endpoint AMQP implementation, causing
// ReceiveEvents to connect but silently never deliver messages.
// The direct per-partition approach uses non-epoch receivers and works reliably.
package iothub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs"
	"go.uber.org/zap"

	"github.com/sentinel-health-engine/telemetry-service/internal/application"
)

// IoTMessage is the JSON payload the mobile app sends to Azure IoT Hub.
// The mobile app MUST match this structure exactly.
type IoTMessage struct {
	DeviceID  string    `json:"deviceId"`
	HeartRate int       `json:"heartRate"` // bpm
	SpO2      float64   `json:"spO2"`      // percentage 0-100
	Timestamp time.Time `json:"timestamp"` // ISO 8601
}

// Consumer reads from the IoT Hub built-in Event Hub endpoint.
type Consumer struct {
	client  *azeventhubs.ConsumerClient
	useCase *application.IngestTelemetryUseCase
	logger  *zap.Logger
}

// NewConsumer creates the IoT Hub consumer.
func NewConsumer(
	eventHubConnStr string,
	eventHubName string,
	consumerGroup string,
	useCase *application.IngestTelemetryUseCase,
	logger *zap.Logger,
) (*Consumer, error) {
	consumerClient, err := azeventhubs.NewConsumerClientFromConnectionString(
		eventHubConnStr,
		eventHubName,
		consumerGroup,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create Event Hub consumer client: %w", err)
	}

	return &Consumer{client: consumerClient, useCase: useCase, logger: logger}, nil
}

// Start begins consuming all partitions. Blocks until ctx is cancelled.
func (c *Consumer) Start(ctx context.Context) error {
	hubProps, err := c.client.GetEventHubProperties(ctx, nil)
	if err != nil {
		return fmt.Errorf("get event hub properties: %w", err)
	}

	c.logger.Info("IoT Hub consumer started — listening for telemetry",
		zap.Strings("partitions", hubProps.PartitionIDs))

	var wg sync.WaitGroup
	for _, pID := range hubProps.PartitionIDs {
		wg.Add(1)
		go func(partitionID string) {
			defer wg.Done()
			c.processPartition(ctx, partitionID)
		}(pID)
	}
	wg.Wait()
	return nil
}

func (c *Consumer) processPartition(ctx context.Context, partitionID string) {
	log := c.logger.With(zap.String("partition", partitionID))
	log.Info("partition consumer started")

	latest := true
	partitionClient, err := c.client.NewPartitionClient(partitionID, &azeventhubs.PartitionClientOptions{
		StartPosition: azeventhubs.StartPosition{Latest: &latest},
	})
	if err != nil {
		log.Error("failed to create partition client", zap.Error(err))
		return
	}
	defer partitionClient.Close(ctx) //nolint:errcheck

	for {
		receiveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		events, err := partitionClient.ReceiveEvents(receiveCtx, 100, nil)
		cancel()

		if err != nil {
			if ctx.Err() != nil {
				return // normal shutdown
			}
			if errors.Is(err, context.DeadlineExceeded) {
				continue // no messages in this window, keep polling
			}
			log.Error("error receiving events", zap.Error(err))
			return
		}

		for _, event := range events {
			c.handleEvent(ctx, event, log)
		}
	}
}

func (c *Consumer) handleEvent(ctx context.Context, event *azeventhubs.ReceivedEventData, log *zap.Logger) {
	var msg IoTMessage
	if err := json.Unmarshal(event.Body, &msg); err != nil {
		log.Warn("skipping malformed IoT message",
			zap.Error(err),
			zap.ByteString("body", event.Body),
		)
		return
	}

	// Fallback: use IoT Hub system property if deviceId not in body
	if msg.DeviceID == "" {
		if devID, ok := event.SystemProperties["iothub-connection-device-id"].(string); ok {
			msg.DeviceID = devID
		}
	}

	// Fallback timestamp
	if msg.Timestamp.IsZero() {
		if event.EnqueuedTime != nil {
			msg.Timestamp = *event.EnqueuedTime
		} else {
			msg.Timestamp = time.Now().UTC()
		}
	}

	cmd := application.IngestTelemetryCommand{
		DeviceID:   msg.DeviceID,
		HeartRate:  msg.HeartRate,
		SpO2:       msg.SpO2,
		MeasuredAt: msg.Timestamp,
		RawPayload: event.Body,
	}

	if err := c.useCase.Execute(ctx, cmd); err != nil {
		log.Error("telemetry ingestion failed", zap.Error(err), zap.String("deviceId", msg.DeviceID))
	}
}
