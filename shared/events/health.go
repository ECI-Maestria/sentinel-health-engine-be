package events

import "time"

const (
	// EventTypeAnomalyDetected is published by Health Rules Service when a reading violates rules.
	EventTypeAnomalyDetected = "health.anomaly_detected"
)

// Severity represents the clinical severity of a detected anomaly.
type Severity string

const (
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
)

// RuleViolation describes a single rule that was violated.
type RuleViolation struct {
	RuleID      string   `json:"ruleId"`
	RuleName    string   `json:"ruleName"`
	MetricName  string   `json:"metricName"`  // "heartRate" or "spO2"
	ActualValue float64  `json:"actualValue"`
	Threshold   float64  `json:"threshold"`
	Operator    string   `json:"operator"` // "GT", "LT", "GTE", "LTE"
	Severity    Severity `json:"severity"`
}

// AnomalyDetectedEvent is published when health rules are violated.
// Alerts Service subscribes to this event.
type AnomalyDetectedEvent struct {
	EventID     string          `json:"eventId"`
	EventType   string          `json:"eventType"`
	OccurredAt  time.Time       `json:"occurredAt"`
	ReadingID   string          `json:"readingId"`
	PatientID   string          `json:"patientId"`
	DeviceID    string          `json:"deviceId"`
	HeartRate   int             `json:"heartRate"`
	SpO2        float64         `json:"spO2"`
	Violations  []RuleViolation `json:"violations"`
	MaxSeverity Severity        `json:"maxSeverity"`
	Timestamp   time.Time       `json:"timestamp"`
}
