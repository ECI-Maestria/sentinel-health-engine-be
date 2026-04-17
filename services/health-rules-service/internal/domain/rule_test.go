package domain_test

import (
	"testing"

	sharedevents "github.com/sentinel-health-engine/shared/events"
	"github.com/sentinel-health-engine/health-rules-service/internal/domain"
)

func TestNewHealthRule_Valid(t *testing.T) {
	r, err := domain.NewHealthRule("id-1", "SpO2 baja", "", domain.MetricSpO2, domain.OperatorLT, 95, sharedevents.SeverityWarning)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID != "id-1" {
		t.Errorf("ID: got %q", r.ID)
	}
	if !r.Active {
		t.Error("new rule should be active by default")
	}
}

func TestNewHealthRule_EmptyID(t *testing.T) {
	_, err := domain.NewHealthRule("", "name", "", domain.MetricHeartRate, domain.OperatorGT, 100, sharedevents.SeverityWarning)
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestNewHealthRule_EmptyName(t *testing.T) {
	_, err := domain.NewHealthRule("id-1", "", "", domain.MetricHeartRate, domain.OperatorGT, 100, sharedevents.SeverityWarning)
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestDefaultRules_Count(t *testing.T) {
	rules := domain.DefaultRules()
	if len(rules) != 6 {
		t.Errorf("expected 6 default rules, got %d", len(rules))
	}
}

func TestDefaultRules_AllActive(t *testing.T) {
	for _, r := range domain.DefaultRules() {
		if !r.Active {
			t.Errorf("rule %q should be active", r.ID)
		}
	}
}

func TestDefaultRules_UniqueIDs(t *testing.T) {
	seen := make(map[string]bool)
	for _, r := range domain.DefaultRules() {
		if seen[r.ID] {
			t.Errorf("duplicate rule ID: %q", r.ID)
		}
		seen[r.ID] = true
	}
}
