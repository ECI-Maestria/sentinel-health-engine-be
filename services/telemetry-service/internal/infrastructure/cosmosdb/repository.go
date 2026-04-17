// Package cosmosdb implements TelemetryRepository using Azure Cosmos DB.
package cosmosdb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"

	"github.com/sentinel-health-engine/telemetry-service/internal/domain"
)

// telemetryDocument is the Cosmos DB document schema.
// Kept separate from the domain aggregate to avoid coupling.
type telemetryDocument struct {
	ID         string  `json:"id"`
	DeviceID   string  `json:"deviceId"`  // partition key
	PatientID  string  `json:"patientId"`
	HeartRate  int     `json:"heartRate"`
	SpO2       float64 `json:"spO2"`
	MeasuredAt string  `json:"measuredAt"`
	ReceivedAt string  `json:"receivedAt"`
	Type       string  `json:"type"` // document type discriminator
}

// TelemetryCosmosRepository implements domain.TelemetryRepository.
type TelemetryCosmosRepository struct {
	client *azcosmos.ContainerClient
}

func NewTelemetryCosmosRepository(client *azcosmos.ContainerClient) *TelemetryCosmosRepository {
	return &TelemetryCosmosRepository{client: client}
}

// Save persists a TelemetryReading to Cosmos DB (upsert by ID).
func (r *TelemetryCosmosRepository) Save(ctx context.Context, reading *domain.TelemetryReading) error {
	doc := telemetryDocument{
		ID:         reading.ID(),
		DeviceID:   reading.DeviceID().String(),
		PatientID:  reading.PatientID().String(),
		HeartRate:  reading.HeartRate().Value(),
		SpO2:       reading.SpO2().Value(),
		MeasuredAt: reading.MeasuredAt().UTC().Format("2006-01-02T15:04:05Z"),
		ReceivedAt: reading.ReceivedAt().UTC().Format("2006-01-02T15:04:05Z"),
		Type:       "TelemetryReading",
	}

	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal telemetry document: %w", err)
	}

	pk := azcosmos.NewPartitionKeyString(reading.DeviceID().String())
	_, err = r.client.UpsertItem(ctx, pk, data, nil)
	if err != nil {
		return fmt.Errorf("cosmos upsert failed: %w", err)
	}
	return nil
}
