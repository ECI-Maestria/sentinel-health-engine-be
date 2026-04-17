package domain_test

import (
	"testing"

	sharedevents "github.com/sentinel-health-engine/shared/events"
	"github.com/sentinel-health-engine/health-rules-service/internal/domain"
)

func rule(id, name string, metric domain.MetricName, op domain.Operator, threshold float64, severity sharedevents.Severity) *domain.HealthRule {
	return &domain.HealthRule{
		ID: id, Name: name, MetricName: metric,
		Operator: op, Threshold: threshold, Severity: severity, Active: true,
	}
}

// --- EvaluateRules ---

func TestEvaluateRules_NoViolations(t *testing.T) {
	rules := []*domain.HealthRule{
		rule("r1", "HR alto", domain.MetricHeartRate, domain.OperatorGT, 100, sharedevents.SeverityWarning),
	}
	input := domain.EvaluationInput{PatientID: "p1", HeartRate: 80, SpO2: 98}
	result := domain.EvaluateRules(rules, input)

	if result.HasAnomalies {
		t.Error("expected no anomalies")
	}
	if len(result.Violations) != 0 {
		t.Errorf("expected 0 violations, got %d", len(result.Violations))
	}
}

func TestEvaluateRules_SingleViolation_GT(t *testing.T) {
	rules := []*domain.HealthRule{
		rule("r1", "HR alto", domain.MetricHeartRate, domain.OperatorGT, 100, sharedevents.SeverityWarning),
	}
	input := domain.EvaluationInput{PatientID: "p1", HeartRate: 120, SpO2: 98}
	result := domain.EvaluateRules(rules, input)

	if !result.HasAnomalies {
		t.Error("expected anomaly")
	}
	if len(result.Violations) != 1 {
		t.Errorf("expected 1 violation, got %d", len(result.Violations))
	}
	if result.MaxSeverity != sharedevents.SeverityWarning {
		t.Errorf("expected WARNING severity, got %s", result.MaxSeverity)
	}
}

func TestEvaluateRules_SingleViolation_LT(t *testing.T) {
	rules := []*domain.HealthRule{
		rule("r1", "SpO2 baja", domain.MetricSpO2, domain.OperatorLT, 95, sharedevents.SeverityWarning),
	}
	input := domain.EvaluationInput{PatientID: "p1", HeartRate: 72, SpO2: 93}
	result := domain.EvaluateRules(rules, input)

	if !result.HasAnomalies {
		t.Error("expected anomaly")
	}
}

func TestEvaluateRules_BoundaryGTE(t *testing.T) {
	rules := []*domain.HealthRule{
		rule("r1", "HR", domain.MetricHeartRate, domain.OperatorGTE, 100, sharedevents.SeverityWarning),
	}
	// Exact threshold — GTE should trigger
	result := domain.EvaluateRules(rules, domain.EvaluationInput{PatientID: "p1", HeartRate: 100})
	if !result.HasAnomalies {
		t.Error("GTE: exact threshold should trigger")
	}
	// One below — should not trigger
	result = domain.EvaluateRules(rules, domain.EvaluationInput{PatientID: "p1", HeartRate: 99})
	if result.HasAnomalies {
		t.Error("GTE: below threshold should not trigger")
	}
}

func TestEvaluateRules_BoundaryLTE(t *testing.T) {
	rules := []*domain.HealthRule{
		rule("r1", "SpO2", domain.MetricSpO2, domain.OperatorLTE, 90, sharedevents.SeverityCritical),
	}
	result := domain.EvaluateRules(rules, domain.EvaluationInput{PatientID: "p1", SpO2: 90})
	if !result.HasAnomalies {
		t.Error("LTE: exact threshold should trigger")
	}
	result = domain.EvaluateRules(rules, domain.EvaluationInput{PatientID: "p1", SpO2: 90.1})
	if result.HasAnomalies {
		t.Error("LTE: above threshold should not trigger")
	}
}

func TestEvaluateRules_MultipleViolations_MaxSeverityCritical(t *testing.T) {
	rules := []*domain.HealthRule{
		rule("r1", "HR warning", domain.MetricHeartRate, domain.OperatorGT, 100, sharedevents.SeverityWarning),
		rule("r2", "HR critical", domain.MetricHeartRate, domain.OperatorGT, 130, sharedevents.SeverityCritical),
	}
	input := domain.EvaluationInput{PatientID: "p1", HeartRate: 150}
	result := domain.EvaluateRules(rules, input)

	if len(result.Violations) != 2 {
		t.Errorf("expected 2 violations, got %d", len(result.Violations))
	}
	if result.MaxSeverity != sharedevents.SeverityCritical {
		t.Errorf("expected CRITICAL, got %s", result.MaxSeverity)
	}
}

func TestEvaluateRules_InactiveRulesSkipped(t *testing.T) {
	rules := []*domain.HealthRule{
		{ID: "r1", Name: "HR", MetricName: domain.MetricHeartRate, Operator: domain.OperatorGT, Threshold: 50, Active: false},
	}
	result := domain.EvaluateRules(rules, domain.EvaluationInput{PatientID: "p1", HeartRate: 150})
	if result.HasAnomalies {
		t.Error("inactive rules should be skipped")
	}
}

func TestEvaluateRules_PatientSpecificRule_WrongPatient(t *testing.T) {
	rules := []*domain.HealthRule{
		{ID: "r1", Name: "HR", MetricName: domain.MetricHeartRate, Operator: domain.OperatorGT,
			Threshold: 50, Severity: sharedevents.SeverityWarning, Active: true, PatientID: "patient-A"},
	}
	result := domain.EvaluateRules(rules, domain.EvaluationInput{PatientID: "patient-B", HeartRate: 150})
	if result.HasAnomalies {
		t.Error("patient-specific rule should not apply to different patient")
	}
}

func TestEvaluateRules_PatientSpecificRule_CorrectPatient(t *testing.T) {
	rules := []*domain.HealthRule{
		{ID: "r1", Name: "HR", MetricName: domain.MetricHeartRate, Operator: domain.OperatorGT,
			Threshold: 50, Severity: sharedevents.SeverityWarning, Active: true, PatientID: "patient-A"},
	}
	result := domain.EvaluateRules(rules, domain.EvaluationInput{PatientID: "patient-A", HeartRate: 150})
	if !result.HasAnomalies {
		t.Error("patient-specific rule should apply to correct patient")
	}
}

func TestEvaluateRules_DefaultRules_NormalValues(t *testing.T) {
	rules := domain.DefaultRules()
	input := domain.EvaluationInput{PatientID: "p1", HeartRate: 72, SpO2: 98}
	result := domain.EvaluateRules(rules, input)
	if result.HasAnomalies {
		t.Errorf("normal vitals should not trigger default rules, got violations: %v", result.Violations)
	}
}

func TestEvaluateRules_DefaultRules_LowSpO2Critical(t *testing.T) {
	rules := domain.DefaultRules()
	input := domain.EvaluationInput{PatientID: "p1", HeartRate: 72, SpO2: 88}
	result := domain.EvaluateRules(rules, input)
	if result.MaxSeverity != sharedevents.SeverityCritical {
		t.Errorf("SpO2=88 should be CRITICAL, got %s", result.MaxSeverity)
	}
}

func TestEvaluateRules_DefaultRules_HighHRCritical(t *testing.T) {
	rules := domain.DefaultRules()
	input := domain.EvaluationInput{PatientID: "p1", HeartRate: 140, SpO2: 98}
	result := domain.EvaluateRules(rules, input)
	if result.MaxSeverity != sharedevents.SeverityCritical {
		t.Errorf("HR=140 should be CRITICAL, got %s", result.MaxSeverity)
	}
}
