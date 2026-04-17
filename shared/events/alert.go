package events

import "time"

const (
	EventTypeAlertCreated = "alert.created"
	EventTypeAlertSent    = "alert.sent"
)

// NotificationChannel represents a delivery channel for alerts.
type NotificationChannel string

const (
	ChannelPush  NotificationChannel = "PUSH"
	ChannelEmail NotificationChannel = "EMAIL"
	ChannelSMS   NotificationChannel = "SMS"
)

// AlertCreatedEvent is published when a new alert is persisted.
type AlertCreatedEvent struct {
	EventID     string    `json:"eventId"`
	EventType   string    `json:"eventType"`
	OccurredAt  time.Time `json:"occurredAt"`
	AlertID     string    `json:"alertId"`
	PatientID   string    `json:"patientId"`
	ReadingID   string    `json:"readingId"`
	MaxSeverity Severity  `json:"maxSeverity"`
	Message     string    `json:"message"`
}
