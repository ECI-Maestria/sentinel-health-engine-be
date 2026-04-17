package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ReminderStatus represents the lifecycle state of a reminder.
type ReminderStatus string

const (
	ReminderPending   ReminderStatus = "PENDING"
	ReminderSent      ReminderStatus = "SENT"
	ReminderCancelled ReminderStatus = "CANCELLED"
)

// Recurrence describes how often a reminder repeats after being sent.
type Recurrence string

const (
	RecurrenceNone    Recurrence = "NONE"
	RecurrenceDaily   Recurrence = "DAILY"
	RecurrenceWeekly  Recurrence = "WEEKLY"
	RecurrenceMonthly Recurrence = "MONTHLY"
)

// Reminder is the domain model for a scheduled patient notification.
type Reminder struct {
	ID         string
	PatientID  string
	CreatedBy  string
	Title      string
	Message    string
	ReminderAt time.Time
	Recurrence Recurrence
	Status     ReminderStatus
	SentAt     *time.Time
	CreatedAt  time.Time
}

// NewReminder creates a validated Reminder value.
func NewReminder(patientID, createdBy, title, message string, reminderAt time.Time, recurrence Recurrence) (*Reminder, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	if patientID == "" {
		return nil, fmt.Errorf("patientID is required")
	}
	if createdBy == "" {
		return nil, fmt.Errorf("createdBy is required")
	}

	return &Reminder{
		ID:         uuid.NewString(),
		PatientID:  patientID,
		CreatedBy:  createdBy,
		Title:      title,
		Message:    message,
		ReminderAt: reminderAt.UTC(),
		Recurrence: recurrence,
		Status:     ReminderPending,
		CreatedAt:  time.Now().UTC(),
	}, nil
}
