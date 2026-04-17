// Package domain contains the Health Rules bounded context domain logic.
// Rules are clinical thresholds applied to biometric data per patient.
package domain

import (
	"errors"

	sharedevents "github.com/sentinel-health-engine/shared/events"
)

// MetricName identifies which biometric metric a rule applies to.
type MetricName string

const (
	MetricHeartRate MetricName = "heartRate"
	MetricSpO2      MetricName = "spO2"
)

// Operator defines the comparison direction for a threshold.
type Operator string

const (
	OperatorGT  Operator = "GT"  // greater than
	OperatorGTE Operator = "GTE" // greater than or equal
	OperatorLT  Operator = "LT"  // less than
	OperatorLTE Operator = "LTE" // less than or equal
)

// HealthRule is a single clinical threshold rule (aggregate root).
type HealthRule struct {
	ID         string
	PatientID  string // empty = applies to all patients
	MetricName MetricName
	Operator   Operator
	Threshold  float64
	Severity   sharedevents.Severity
	Name       string
	Active     bool
}

func NewHealthRule(id, name, patientID string, metric MetricName, op Operator, threshold float64, severity sharedevents.Severity) (*HealthRule, error) {
	if id == "" {
		return nil, errors.New("rule id cannot be empty")
	}
	if name == "" {
		return nil, errors.New("rule name cannot be empty")
	}
	return &HealthRule{
		ID: id, Name: name, PatientID: patientID,
		MetricName: metric, Operator: op, Threshold: threshold,
		Severity: severity, Active: true,
	}, nil
}

// DefaultRules returns the baseline clinical rules for all patients.
// These represent standard medical thresholds for SpO2 and heart rate.
// A physician can add per-patient overrides in a future iteration.
func DefaultRules() []*HealthRule {
	return []*HealthRule{
		{ID: "default-spo2-warning",  Name: "SpO2 Baja (Advertencia)",  MetricName: MetricSpO2,      Operator: OperatorLT, Threshold: 95.0, Severity: sharedevents.SeverityWarning,  Active: true},
		{ID: "default-spo2-critical", Name: "SpO2 Crítica",              MetricName: MetricSpO2,      Operator: OperatorLT, Threshold: 90.0, Severity: sharedevents.SeverityCritical, Active: true},
		{ID: "default-hr-high-warn",  Name: "Taquicardia (Advertencia)", MetricName: MetricHeartRate, Operator: OperatorGT, Threshold: 100,  Severity: sharedevents.SeverityWarning,  Active: true},
		{ID: "default-hr-high-crit",  Name: "Taquicardia Severa",        MetricName: MetricHeartRate, Operator: OperatorGT, Threshold: 130,  Severity: sharedevents.SeverityCritical, Active: true},
		{ID: "default-hr-low-warn",   Name: "Bradicardia (Advertencia)", MetricName: MetricHeartRate, Operator: OperatorLT, Threshold: 50,   Severity: sharedevents.SeverityWarning,  Active: true},
		{ID: "default-hr-low-crit",   Name: "Bradicardia Severa",        MetricName: MetricHeartRate, Operator: OperatorLT, Threshold: 40,   Severity: sharedevents.SeverityCritical, Active: true},
	}
}
