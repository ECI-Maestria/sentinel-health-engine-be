// Package domain contains the Alerts bounded context domain logic.
package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	sharedevents "github.com/sentinel-health-engine/shared/events"
)

// AlertStatus represents the lifecycle state of an alert.
type AlertStatus string

const (
	AlertStatusCreated   AlertStatus = "CREATED"
	AlertStatusSent      AlertStatus = "SENT"
	AlertStatusConfirmed AlertStatus = "CONFIRMED"
	AlertStatusDiscarded AlertStatus = "DISCARDED"
)

// Alert is the aggregate root for the Alerts bounded context.
// Tracks a detected anomaly and its full notification lifecycle.
type Alert struct {
	id         string
	patientID  string
	readingID  string
	message    string
	severity   sharedevents.Severity
	violations []sharedevents.RuleViolation
	status     AlertStatus
	createdAt  time.Time
	sentAt     *time.Time
}

// NewAlert creates an Alert from an AnomalyDetectedEvent.
func NewAlert(event sharedevents.AnomalyDetectedEvent) (*Alert, error) {
	if event.PatientID == "" {
		return nil, errors.New("patientID cannot be empty")
	}
	if len(event.Violations) == 0 {
		return nil, errors.New("alert must have at least one violation")
	}
	return &Alert{
		id:         uuid.New().String(),
		patientID:  event.PatientID,
		readingID:  event.ReadingID,
		message:    buildMessage(event),
		severity:   event.MaxSeverity,
		violations: event.Violations,
		status:     AlertStatusCreated,
		createdAt:  time.Now().UTC(),
	}, nil
}

// MarkSent transitions the alert to SENT state.
func (a *Alert) MarkSent() {
	now := time.Now().UTC()
	a.sentAt = &now
	a.status = AlertStatusSent
}

func (a *Alert) ID() string                               { return a.id }
func (a *Alert) PatientID() string                        { return a.patientID }
func (a *Alert) ReadingID() string                        { return a.readingID }
func (a *Alert) Message() string                          { return a.message }
func (a *Alert) Severity() sharedevents.Severity          { return a.severity }
func (a *Alert) Violations() []sharedevents.RuleViolation { return a.violations }
func (a *Alert) Status() AlertStatus                      { return a.status }
func (a *Alert) CreatedAt() time.Time                     { return a.createdAt }

func buildMessage(event sharedevents.AnomalyDetectedEvent) string {
	prefix := "⚠️ Advertencia"
	if event.MaxSeverity == sharedevents.SeverityCritical {
		prefix = "🚨 ALERTA CRÍTICA"
	}
	msg := prefix + " — Signos vitales fuera de rango:\n"
	for _, v := range event.Violations {
		msg += "• " + v.RuleName + ": " + formatMetric(v.MetricName, v.ActualValue) + "\n"
	}
	return msg
}

func formatMetric(metric string, val float64) string {
	switch metric {
	case "heartRate":
		return fmt.Sprintf("%.0f bpm", val)
	case "spO2":
		return fmt.Sprintf("%.1f%%", val)
	}
	return fmt.Sprintf("%.2f", val)
}
