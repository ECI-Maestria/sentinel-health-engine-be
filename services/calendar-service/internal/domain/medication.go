package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Frequency describes how often a medication should be taken.
type Frequency string

const (
	FrequencyDaily      Frequency = "DAILY"
	FrequencyTwiceDaily Frequency = "TWICE_DAILY"
	FrequencyThreeTimes Frequency = "THREE_TIMES_DAILY"
	FrequencyWeekly     Frequency = "WEEKLY"
	FrequencyAsNeeded   Frequency = "AS_NEEDED"
)

// Medication is the domain model for a patient's prescribed medication.
type Medication struct {
	ID             string
	PatientID      string
	PrescribedBy   string
	Name           string
	Dosage         string
	Frequency      Frequency
	ScheduledTimes []string
	StartDate      time.Time
	EndDate        *time.Time
	Notes          string
	IsActive       bool
	CreatedAt      time.Time
}

// NewMedication creates a validated Medication value.
func NewMedication(patientID, prescribedBy, name, dosage string, freq Frequency, times []string, startDate time.Time) (*Medication, error) {
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if dosage == "" {
		return nil, fmt.Errorf("dosage is required")
	}
	if patientID == "" {
		return nil, fmt.Errorf("patientID is required")
	}
	if prescribedBy == "" {
		return nil, fmt.Errorf("prescribedBy is required")
	}

	scheduledTimes := times
	if scheduledTimes == nil {
		scheduledTimes = []string{}
	}

	return &Medication{
		ID:             uuid.NewString(),
		PatientID:      patientID,
		PrescribedBy:   prescribedBy,
		Name:           name,
		Dosage:         dosage,
		Frequency:      freq,
		ScheduledTimes: scheduledTimes,
		StartDate:      startDate.UTC(),
		IsActive:       true,
		CreatedAt:      time.Now().UTC(),
	}, nil
}
