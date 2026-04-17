package svcinternal

import (
	"context"
	"fmt"

	domaincart "github.com/sentinel-health-engine/user-service/internal/domain/caretaker"
	domaindevice "github.com/sentinel-health-engine/user-service/internal/domain/device"
	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// Contact holds the notification coordinates for one person (patient or caretaker).
// FCMToken is from the most recently active device; empty if no device is registered.
type Contact struct {
	Email    string
	FCMToken string
}

// GetPatientContactsUseCase returns all notification contacts for a patient.
// Contacts include: the patient + all linked caretakers.
// Used by alerts-service to send push and email notifications.
type GetPatientContactsUseCase struct {
	users      user.Repository
	devices    domaindevice.Repository
	caretakers domaincart.Repository
}

func NewGetPatientContactsUseCase(
	users user.Repository,
	devices domaindevice.Repository,
	caretakers domaincart.Repository,
) *GetPatientContactsUseCase {
	return &GetPatientContactsUseCase{users: users, devices: devices, caretakers: caretakers}
}

// Execute returns contacts for the patient and all linked caretakers.
func (uc *GetPatientContactsUseCase) Execute(ctx context.Context, patientID string) ([]Contact, error) {
	patient, err := uc.users.FindByID(ctx, patientID)
	if err != nil {
		return nil, fmt.Errorf("patient not found: %s", patientID)
	}

	var contacts []Contact

	// Add patient as first contact.
	contacts = append(contacts, Contact{
		Email:    patient.Email(),
		FCMToken: uc.latestFCMToken(ctx, patient.ID()),
	})

	// Add all linked caretakers.
	rels, err := uc.caretakers.ListCaretakersByPatient(ctx, patientID)
	if err != nil {
		return contacts, nil // return patient contact even if caretaker lookup fails
	}

	for _, rel := range rels {
		u, err := uc.users.FindByID(ctx, rel.CaretakerID())
		if err != nil || !u.IsActive() {
			continue
		}
		contacts = append(contacts, Contact{
			Email:    u.Email(),
			FCMToken: uc.latestFCMToken(ctx, u.ID()),
		})
	}

	return contacts, nil
}

// latestFCMToken returns the FCM token of the most recently active device for a user.
// Returns empty string if no active device with a token exists.
func (uc *GetPatientContactsUseCase) latestFCMToken(ctx context.Context, userID string) string {
	devs, err := uc.devices.ListActiveByUserID(ctx, userID)
	if err != nil || len(devs) == 0 {
		return ""
	}
	// ListActiveByUserID returns devices ordered by last_seen_at DESC.
	return devs[0].FCMToken()
}
