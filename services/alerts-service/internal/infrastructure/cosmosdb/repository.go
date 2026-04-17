package cosmosdb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"

	"github.com/sentinel-health-engine/alerts-service/internal/domain"
	sharedevents "github.com/sentinel-health-engine/shared/events"
)

type alertDocument struct {
	ID         string                       `json:"id"`
	PatientID  string                       `json:"patientId"` // partition key
	ReadingID  string                       `json:"readingId"`
	Message    string                       `json:"message"`
	Severity   sharedevents.Severity        `json:"severity"`
	Violations []sharedevents.RuleViolation `json:"violations"`
	Status     domain.AlertStatus           `json:"status"`
	CreatedAt  time.Time                    `json:"createdAt"`
	Type       string                       `json:"type"`
}

// AlertCosmosRepository implements domain.AlertRepository.
type AlertCosmosRepository struct {
	client *azcosmos.ContainerClient
}

func NewAlertCosmosRepository(client *azcosmos.ContainerClient) *AlertCosmosRepository {
	return &AlertCosmosRepository{client: client}
}

func (r *AlertCosmosRepository) Save(ctx context.Context, alert *domain.Alert) error {
	return r.upsert(ctx, alert)
}

func (r *AlertCosmosRepository) Update(ctx context.Context, alert *domain.Alert) error {
	return r.upsert(ctx, alert)
}

func (r *AlertCosmosRepository) upsert(ctx context.Context, alert *domain.Alert) error {
	doc := alertDocument{
		ID:         alert.ID(),
		PatientID:  alert.PatientID(),
		ReadingID:  alert.ReadingID(),
		Message:    alert.Message(),
		Severity:   alert.Severity(),
		Violations: alert.Violations(),
		Status:     alert.Status(),
		CreatedAt:  alert.CreatedAt(),
		Type:       "Alert",
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal alert: %w", err)
	}
	pk := azcosmos.NewPartitionKeyString(alert.PatientID())
	_, err = r.client.UpsertItem(ctx, pk, data, nil)
	return err
}
