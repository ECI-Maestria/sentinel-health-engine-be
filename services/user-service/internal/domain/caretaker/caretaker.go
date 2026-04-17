// Package caretaker contains the PatientCaretaker relationship aggregate.
package caretaker

import (
	"fmt"
	"time"
)

// PatientCaretaker represents the relationship between a patient and a caretaker.
// This is a value object — it has no identity beyond the composite key (patientID, caretakerID).
type PatientCaretaker struct {
	patientID   string
	caretakerID string
	linkedBy    string // ID of the user who created the link (doctor or patient)
	createdAt   time.Time
}

// NewPatientCaretaker creates the relationship enforcing domain invariants.
func NewPatientCaretaker(patientID, caretakerID, linkedBy string) (*PatientCaretaker, error) {
	if patientID == "" {
		return nil, fmt.Errorf("patient id is required")
	}
	if caretakerID == "" {
		return nil, fmt.Errorf("caretaker id is required")
	}
	if linkedBy == "" {
		return nil, fmt.Errorf("linked_by is required")
	}
	if patientID == caretakerID {
		return nil, fmt.Errorf("a patient cannot be their own caretaker")
	}

	return &PatientCaretaker{
		patientID:   patientID,
		caretakerID: caretakerID,
		linkedBy:    linkedBy,
		createdAt:   time.Now().UTC(),
	}, nil
}

// Reconstitute rebuilds the relationship from persisted data.
func Reconstitute(patientID, caretakerID, linkedBy string, createdAt time.Time) *PatientCaretaker {
	return &PatientCaretaker{
		patientID:   patientID,
		caretakerID: caretakerID,
		linkedBy:    linkedBy,
		createdAt:   createdAt,
	}
}

func (pc *PatientCaretaker) PatientID() string   { return pc.patientID }
func (pc *PatientCaretaker) CaretakerID() string { return pc.caretakerID }
func (pc *PatientCaretaker) LinkedBy() string    { return pc.linkedBy }
func (pc *PatientCaretaker) CreatedAt() time.Time { return pc.createdAt }
