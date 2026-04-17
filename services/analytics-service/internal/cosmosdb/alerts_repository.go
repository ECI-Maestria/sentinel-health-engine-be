package cosmosdb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
)

// RuleViolation describes a single rule that was violated to produce an alert.
type RuleViolation struct {
	RuleName    string  `json:"ruleName"`
	MetricName  string  `json:"metricName"`
	ActualValue float64 `json:"actualValue"`
}

// AlertRecord represents an alert document stored in Cosmos DB.
type AlertRecord struct {
	ID         string          `json:"id"`
	PatientID  string          `json:"patientId"`
	ReadingID  string          `json:"readingId"`
	Message    string          `json:"message"`
	Severity   string          `json:"severity"`
	Status     string          `json:"status"`
	Violations []RuleViolation `json:"violations"`
	CreatedAt  time.Time       `json:"createdAt"`
}

// AlertStats aggregates alert counts for a patient in a time range.
type AlertStats struct {
	Total       int        `json:"total"`
	Warning     int        `json:"warning"`
	Critical    int        `json:"critical"`
	LastAlertAt *time.Time `json:"lastAlertAt,omitempty"`
}

// AlertsRepository provides query access to the alerts Cosmos DB container.
type AlertsRepository struct {
	container *azcosmos.ContainerClient
}

// NewAlertsRepository creates a new AlertsRepository using the given container client.
func NewAlertsRepository(container *azcosmos.ContainerClient) *AlertsRepository {
	return &AlertsRepository{container: container}
}

// GetHistory returns alerts for a patient within the given time range. An optional
// severity filter (e.g. "WARNING" or "CRITICAL") can be supplied; pass an empty
// string to skip the filter. Alerts are returned ordered by createdAt descending.
// Uses a partition-key query because the alerts container is partitioned by patientId.
func (r *AlertsRepository) GetHistory(ctx context.Context, patientID string, from, to time.Time, severity string) ([]AlertRecord, error) {
	var sb strings.Builder
	params := []azcosmos.QueryParameter{
		{Name: "@patientId", Value: patientID},
		{Name: "@from", Value: from.UTC().Format(time.RFC3339)},
		{Name: "@to", Value: to.UTC().Format(time.RFC3339)},
	}

	sb.WriteString("SELECT * FROM c WHERE c.patientId = @patientId AND c.createdAt >= @from AND c.createdAt <= @to")

	if severity != "" {
		sb.WriteString(" AND c.severity = @severity")
		params = append(params, azcosmos.QueryParameter{Name: "@severity", Value: strings.ToUpper(severity)})
	}

	sb.WriteString(" ORDER BY c.createdAt DESC")

	queryOptions := &azcosmos.QueryOptions{
		QueryParameters: params,
	}

	pk := azcosmos.NewPartitionKeyString(patientID)
	pager := r.container.NewQueryItemsPager(sb.String(), pk, queryOptions)

	var alerts []AlertRecord
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("querying alerts history: %w", err)
		}
		for _, itemBytes := range page.Items {
			var alert AlertRecord
			if err := json.Unmarshal(itemBytes, &alert); err != nil {
				return nil, fmt.Errorf("unmarshalling alert record: %w", err)
			}
			alerts = append(alerts, alert)
		}
	}

	return alerts, nil
}

// GetStats counts alerts by severity for a patient within the given time range.
// It also records the timestamp of the most recent alert.
func (r *AlertsRepository) GetStats(ctx context.Context, patientID string, from, to time.Time) (AlertStats, error) {
	alerts, err := r.GetHistory(ctx, patientID, from, to, "")
	if err != nil {
		return AlertStats{}, fmt.Errorf("fetching alerts for stats: %w", err)
	}

	stats := AlertStats{}
	for i, a := range alerts {
		stats.Total++
		switch strings.ToUpper(a.Severity) {
		case "WARNING":
			stats.Warning++
		case "CRITICAL":
			stats.Critical++
		}
		// alerts are ordered by createdAt DESC so the first item is the latest
		if i == 0 {
			t := a.CreatedAt
			stats.LastAlertAt = &t
		}
	}

	return stats, nil
}

// AcknowledgeAlert updates the status of an alert document to "ACKNOWLEDGED".
// The alerts container is partitioned by patientId, so we use that as the partition key.
func (r *AlertsRepository) AcknowledgeAlert(ctx context.Context, patientID, alertID string) error {
	pk := azcosmos.NewPartitionKeyString(patientID)

	ops := azcosmos.PatchOperations{}
	ops.AppendReplace("/status", "ACKNOWLEDGED")

	_, err := r.container.PatchItem(ctx, pk, alertID, ops, nil)
	if err != nil {
		return fmt.Errorf("acknowledging alert %s: %w", alertID, err)
	}
	return nil
}
