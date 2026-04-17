package cosmosdb

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

// VitalReading represents a telemetry reading stored in Cosmos DB.
type VitalReading struct {
	ID         string    `json:"id"`
	DeviceID   string    `json:"deviceId"`
	PatientID  string    `json:"patientId"`
	HeartRate  int       `json:"heartRate"`
	SpO2       float64   `json:"spO2"`
	MeasuredAt time.Time `json:"measuredAt"`
	ReceivedAt time.Time `json:"receivedAt"`
}

// VitalsRepository provides query access to the telemetry Cosmos DB container.
type VitalsRepository struct {
	container *azcosmos.ContainerClient
}

// NewVitalsRepository creates a new VitalsRepository using the given container client.
func NewVitalsRepository(container *azcosmos.ContainerClient) *VitalsRepository {
	return &VitalsRepository{container: container}
}

// GetHistory returns all telemetry readings for a patient within the given time range,
// ordered by measuredAt descending. Uses a cross-partition query (no ORDER BY in Cosmos
// because the Go SDK does not implement client-side partition merge for ORDER BY) and
// sorts the results in memory.
func (r *VitalsRepository) GetHistory(ctx context.Context, patientID string, from, to time.Time) ([]VitalReading, error) {
	// No ORDER BY: cross-partition ORDER BY requires client-side merge which the Go SDK
	// does not support. We sort in memory after fetching.
	query := "SELECT * FROM c WHERE c.patientId = @patientId AND c.measuredAt >= @from AND c.measuredAt <= @to"

	queryOptions := &azcosmos.QueryOptions{
		QueryParameters: []azcosmos.QueryParameter{
			{Name: "@patientId", Value: patientID},
			{Name: "@from", Value: from.UTC().Format(time.RFC3339)},
			{Name: "@to", Value: to.UTC().Format(time.RFC3339)},
		},
	}

	readings, err := r.runQuery(ctx, query, queryOptions)
	if err != nil {
		return nil, fmt.Errorf("querying vitals history: %w", err)
	}

	sort.Slice(readings, func(i, j int) bool {
		return readings[i].MeasuredAt.After(readings[j].MeasuredAt)
	})

	return readings, nil
}

// GetLatest returns the most recent telemetry reading for the given patient.
// Returns nil without error when no reading is found.
func (r *VitalsRepository) GetLatest(ctx context.Context, patientID string) (*VitalReading, error) {
	// No TOP 1 / ORDER BY: fetch all and find max in memory (same cross-partition limitation).
	query := "SELECT * FROM c WHERE c.patientId = @patientId"

	queryOptions := &azcosmos.QueryOptions{
		QueryParameters: []azcosmos.QueryParameter{
			{Name: "@patientId", Value: patientID},
		},
	}

	readings, err := r.runQuery(ctx, query, queryOptions)
	if err != nil {
		return nil, fmt.Errorf("querying latest vital: %w", err)
	}

	if len(readings) == 0 {
		return nil, nil
	}

	latest := &readings[0]
	for i := 1; i < len(readings); i++ {
		if readings[i].MeasuredAt.After(latest.MeasuredAt) {
			latest = &readings[i]
		}
	}

	return latest, nil
}

// runQuery executes the given parameterized query against the telemetry container
// using an empty partition key (cross-partition).
func (r *VitalsRepository) runQuery(ctx context.Context, query string, opts *azcosmos.QueryOptions) ([]VitalReading, error) {
	pager := r.container.NewQueryItemsPager(query, azcosmos.PartitionKey{}, opts)

	var readings []VitalReading
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, itemBytes := range page.Items {
			var reading VitalReading
			if err := json.Unmarshal(itemBytes, &reading); err != nil {
				return nil, fmt.Errorf("unmarshalling vital reading: %w", err)
			}
			readings = append(readings, reading)
		}
	}

	return readings, nil
}
