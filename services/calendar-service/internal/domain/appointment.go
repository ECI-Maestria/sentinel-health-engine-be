package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AppointmentStatus represents the lifecycle state of an appointment.
type AppointmentStatus string

const (
	StatusScheduled AppointmentStatus = "SCHEDULED"
	StatusCompleted AppointmentStatus = "COMPLETED"
	StatusCancelled AppointmentStatus = "CANCELLED"
)

// Appointment is the core domain model for a scheduled patient-doctor meeting.
type Appointment struct {
	ID             string
	PatientID      string
	DoctorID       string
	Title          string
	ScheduledAt    time.Time
	Location       string
	Notes          string
	Status         AppointmentStatus
	ReminderSentAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewAppointment creates a validated Appointment value. ScheduledAt must be in the future.
func NewAppointment(patientID, doctorID, title string, scheduledAt time.Time, location, notes string) (*Appointment, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if scheduledAt.Before(time.Now().UTC()) {
		return nil, fmt.Errorf("scheduledAt must be in the future")
	}

	now := time.Now().UTC()
	return &Appointment{
		ID:          uuid.NewString(),
		PatientID:   patientID,
		DoctorID:    doctorID,
		Title:       title,
		ScheduledAt: scheduledAt.UTC(),
		Location:    location,
		Notes:       notes,
		Status:      StatusScheduled,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
