package userservice

import (
	"context"

	"github.com/sentinel-health-engine/alerts-service/internal/application"
)

// ContactResolverAdapter adapts ContactResolver to the application.ContactResolver interface.
type ContactResolverAdapter struct {
	resolver *ContactResolver
}

// NewContactResolverAdapter wraps ContactResolver to satisfy the application port.
func NewContactResolverAdapter(resolver *ContactResolver) *ContactResolverAdapter {
	return &ContactResolverAdapter{resolver: resolver}
}

// GetContacts calls the user-service and maps results to application.PatientContact.
func (a *ContactResolverAdapter) GetContacts(ctx context.Context, patientID string) ([]application.PatientContact, error) {
	contacts, err := a.resolver.GetContacts(ctx, patientID)
	if err != nil {
		return nil, err
	}

	result := make([]application.PatientContact, 0, len(contacts))
	for _, c := range contacts {
		result = append(result, application.PatientContact{
			PatientID: patientID,
			FCMToken:  c.FCMToken,
			Email:     c.Email,
		})
	}
	return result, nil
}
