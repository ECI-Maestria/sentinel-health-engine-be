package domain_test

import (
	"strings"
	"testing"

	sharedevents "github.com/sentinel-health-engine/shared/events"
	"github.com/sentinel-health-engine/alerts-service/internal/domain"
)

func anomalyEvent(patientID string, severity sharedevents.Severity, violations []sharedevents.RuleViolation) sharedevents.AnomalyDetectedEvent {
	return sharedevents.AnomalyDetectedEvent{
		PatientID:   patientID,
		ReadingID:   "reading-001",
		MaxSeverity: severity,
		Violations:  violations,
		HeartRate:   72,
		SpO2:        98.0,
	}
}

func oneViolation(metric string, actual float64) []sharedevents.RuleViolation {
	return []sharedevents.RuleViolation{
		{RuleID: "r1", RuleName: "Test Rule", MetricName: metric, ActualValue: actual, Threshold: 95, Operator: "LT"},
	}
}

// --- NewAlert ---

func TestNewAlert_Valid(t *testing.T) {
	event := anomalyEvent("patient-01", sharedevents.SeverityWarning, oneViolation("spO2", 93))
	a, err := domain.NewAlert(event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID() == "" {
		t.Error("ID should not be empty")
	}
	if a.PatientID() != "patient-01" {
		t.Errorf("PatientID: got %q", a.PatientID())
	}
	if a.Status() != domain.AlertStatusCreated {
		t.Errorf("Status: expected CREATED, got %q", a.Status())
	}
	if a.CreatedAt().IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestNewAlert_EmptyPatientID(t *testing.T) {
	event := anomalyEvent("", sharedevents.SeverityWarning, oneViolation("spO2", 93))
	_, err := domain.NewAlert(event)
	if err == nil {
		t.Fatal("expected error for empty patientID")
	}
}

func TestNewAlert_NoViolations(t *testing.T) {
	event := anomalyEvent("patient-01", sharedevents.SeverityWarning, []sharedevents.RuleViolation{})
	_, err := domain.NewAlert(event)
	if err == nil {
		t.Fatal("expected error when violations list is empty")
	}
}

func TestNewAlert_UniqueIDs(t *testing.T) {
	event := anomalyEvent("patient-01", sharedevents.SeverityWarning, oneViolation("spO2", 93))
	a1, _ := domain.NewAlert(event)
	a2, _ := domain.NewAlert(event)
	if a1.ID() == a2.ID() {
		t.Error("consecutive alerts should have different IDs")
	}
}

// --- MarkSent ---

func TestMarkSent_TransitionsStatus(t *testing.T) {
	event := anomalyEvent("patient-01", sharedevents.SeverityWarning, oneViolation("spO2", 93))
	a, _ := domain.NewAlert(event)

	a.MarkSent()
	if a.Status() != domain.AlertStatusSent {
		t.Errorf("expected SENT, got %q", a.Status())
	}
}

// --- Message content ---

func TestNewAlert_MessageContainsWarningPrefix(t *testing.T) {
	event := anomalyEvent("patient-01", sharedevents.SeverityWarning, oneViolation("spO2", 93))
	a, _ := domain.NewAlert(event)
	if !strings.Contains(a.Message(), "Advertencia") {
		t.Errorf("WARNING message should contain 'Advertencia', got: %q", a.Message())
	}
}

func TestNewAlert_MessageContainsCriticalPrefix(t *testing.T) {
	event := anomalyEvent("patient-01", sharedevents.SeverityCritical, oneViolation("heartRate", 150))
	a, _ := domain.NewAlert(event)
	if !strings.Contains(a.Message(), "CRÍTICA") {
		t.Errorf("CRITICAL message should contain 'CRÍTICA', got: %q", a.Message())
	}
}

func TestNewAlert_MessageFormatsHeartRate(t *testing.T) {
	event := anomalyEvent("patient-01", sharedevents.SeverityWarning, oneViolation("heartRate", 140))
	a, _ := domain.NewAlert(event)
	if !strings.Contains(a.Message(), "bpm") {
		t.Errorf("heart rate message should contain 'bpm', got: %q", a.Message())
	}
}

func TestNewAlert_MessageFormatsSpO2(t *testing.T) {
	event := anomalyEvent("patient-01", sharedevents.SeverityWarning, oneViolation("spO2", 93.5))
	a, _ := domain.NewAlert(event)
	if !strings.Contains(a.Message(), "%") {
		t.Errorf("SpO2 message should contain '%%', got: %q", a.Message())
	}
}

func TestNewAlert_MessageContainsRuleName(t *testing.T) {
	violations := []sharedevents.RuleViolation{
		{RuleID: "r1", RuleName: "SpO2 Crítica", MetricName: "spO2", ActualValue: 88},
	}
	event := anomalyEvent("patient-01", sharedevents.SeverityCritical, violations)
	a, _ := domain.NewAlert(event)
	if !strings.Contains(a.Message(), "SpO2 Crítica") {
		t.Errorf("message should contain rule name, got: %q", a.Message())
	}
}
