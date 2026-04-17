package caretaker

import (
	"context"
	"fmt"

	domaincart "github.com/sentinel-health-engine/user-service/internal/domain/caretaker"
	"github.com/sentinel-health-engine/user-service/internal/domain/user"
	"go.uber.org/zap"
)

// PatientSummary bundles the patient user with their linked-since timestamp.
type PatientSummary struct {
	User         *user.User
	Relationship *domaincart.PatientCaretaker
}

// GetMyPatientsUseCase returns all patients linked to a caretaker.
// An empty slice means the caretaker is not yet assigned to any patient.
type GetMyPatientsUseCase struct {
	caretakers domaincart.Repository
	users      user.Repository
	logger     *zap.Logger
}

func NewGetMyPatientsUseCase(caretakers domaincart.Repository, users user.Repository, logger *zap.Logger) *GetMyPatientsUseCase {
	return &GetMyPatientsUseCase{caretakers: caretakers, users: users, logger: logger}
}

func (uc *GetMyPatientsUseCase) Execute(ctx context.Context, caretakerID string) ([]PatientSummary, error) {
	relationships, err := uc.caretakers.ListPatientsByCaretaker(ctx, caretakerID)
	if err != nil {
		return nil, fmt.Errorf("list patients: %w", err)
	}

	result := make([]PatientSummary, 0, len(relationships))
	for _, rel := range relationships {
		u, err := uc.users.FindByID(ctx, rel.PatientID())
		if err != nil {
			// Patient deleted — skip silently.
			uc.logger.Warn("patient referenced in caretaker relationship not found",
				zap.String("patientId", rel.PatientID()),
				zap.String("caretakerId", caretakerID),
			)
			continue
		}
		result = append(result, PatientSummary{User: u, Relationship: rel})
	}
	return result, nil
}
