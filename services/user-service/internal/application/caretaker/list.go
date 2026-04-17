package caretaker

import (
	"context"
	"fmt"

	domaincart "github.com/sentinel-health-engine/user-service/internal/domain/caretaker"
	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// CaretakerWithUser bundles the relationship with the caretaker's user profile.
type CaretakerWithUser struct {
	Relationship *domaincart.PatientCaretaker
	User         *user.User
}

// ListCaretakersUseCase returns all caretakers linked to a patient.
type ListCaretakersUseCase struct {
	caretakers domaincart.Repository
	users      user.Repository
}

func NewListCaretakersUseCase(caretakers domaincart.Repository, users user.Repository) *ListCaretakersUseCase {
	return &ListCaretakersUseCase{caretakers: caretakers, users: users}
}

// Execute returns caretakers with their user profiles for a given patient.
func (uc *ListCaretakersUseCase) Execute(ctx context.Context, patientID string) ([]*CaretakerWithUser, error) {
	rels, err := uc.caretakers.ListCaretakersByPatient(ctx, patientID)
	if err != nil {
		return nil, fmt.Errorf("list caretakers: %w", err)
	}

	result := make([]*CaretakerWithUser, 0, len(rels))
	for _, rel := range rels {
		u, err := uc.users.FindByID(ctx, rel.CaretakerID())
		if err != nil {
			continue // skip if user was deleted
		}
		result = append(result, &CaretakerWithUser{Relationship: rel, User: u})
	}
	return result, nil
}
