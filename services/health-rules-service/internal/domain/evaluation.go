package domain

import sharedevents "github.com/sentinel-health-engine/shared/events"

// EvaluationInput contains biometric values to evaluate against rules.
type EvaluationInput struct {
	PatientID string
	DeviceID  string
	ReadingID string
	HeartRate float64
	SpO2      float64
}

// EvaluationResult holds the outcome of evaluating all rules.
type EvaluationResult struct {
	HasAnomalies bool
	Violations   []sharedevents.RuleViolation
	MaxSeverity  sharedevents.Severity
}

// EvaluateRules applies all active rules to the input and returns all violations.
// Rules are evaluated independently — multiple violations can coexist.
func EvaluateRules(rules []*HealthRule, input EvaluationInput) EvaluationResult {
	var violations []sharedevents.RuleViolation
	maxSeverity := sharedevents.Severity("")

	for _, rule := range rules {
		if !rule.Active {
			continue
		}
		// Skip patient-specific rules that don't match this patient
		if rule.PatientID != "" && rule.PatientID != input.PatientID {
			continue
		}

		var actualValue float64
		switch rule.MetricName {
		case MetricHeartRate:
			actualValue = input.HeartRate
		case MetricSpO2:
			actualValue = input.SpO2
		default:
			continue
		}

		if violates(actualValue, rule.Operator, rule.Threshold) {
			violations = append(violations, sharedevents.RuleViolation{
				RuleID:      rule.ID,
				RuleName:    rule.Name,
				MetricName:  string(rule.MetricName),
				ActualValue: actualValue,
				Threshold:   rule.Threshold,
				Operator:    string(rule.Operator),
				Severity:    rule.Severity,
			})
			maxSeverity = higherSeverity(maxSeverity, rule.Severity)
		}
	}

	return EvaluationResult{
		HasAnomalies: len(violations) > 0,
		Violations:   violations,
		MaxSeverity:  maxSeverity,
	}
}

func violates(actual float64, op Operator, threshold float64) bool {
	switch op {
	case OperatorGT:
		return actual > threshold
	case OperatorGTE:
		return actual >= threshold
	case OperatorLT:
		return actual < threshold
	case OperatorLTE:
		return actual <= threshold
	}
	return false
}

func higherSeverity(a, b sharedevents.Severity) sharedevents.Severity {
	if b == sharedevents.SeverityCritical || a == sharedevents.SeverityCritical {
		return sharedevents.SeverityCritical
	}
	if a == sharedevents.SeverityWarning || b == sharedevents.SeverityWarning {
		return sharedevents.SeverityWarning
	}
	return b
}
