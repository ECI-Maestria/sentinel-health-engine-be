// Package caretaker contains use cases for managing patient–caretaker relationships.
package caretaker

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	domaincart "github.com/sentinel-health-engine/user-service/internal/domain/caretaker"
	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// LinkCaretakerCommand is the input DTO.
// Provide either CaretakerID (UUID) or CaretakerEmail — not both are required.
// If CaretakerEmail is set and CaretakerID is empty, the caretaker is resolved by email.
type LinkCaretakerCommand struct {
	PatientID      string
	CaretakerID    string // UUID — optional if CaretakerEmail is set
	CaretakerEmail string // email — optional if CaretakerID is set
	LinkedBy       string // ID of the requesting user (doctor or patient)
}

// LinkCaretakerUseCase creates a patient–caretaker relationship.
type LinkCaretakerUseCase struct {
	caretakers domaincart.Repository
	users      user.Repository
	logger     *zap.Logger
}

func NewLinkCaretakerUseCase(caretakers domaincart.Repository, users user.Repository, logger *zap.Logger) *LinkCaretakerUseCase {
	return &LinkCaretakerUseCase{caretakers: caretakers, users: users, logger: logger}
}

// Execute validates both sides of the relationship and persists it.
// The caretaker can be resolved by ID or by email (one of the two must be provided).
func (uc *LinkCaretakerUseCase) Execute(ctx context.Context, cmd LinkCaretakerCommand) error {
	if cmd.CaretakerID == "" && cmd.CaretakerEmail == "" {
		return fmt.Errorf("caretakerId or caretakerEmail is required")
	}

	patient, err := uc.users.FindByID(ctx, cmd.PatientID)
	if err != nil || patient.Role() != user.RolePatient {
		return fmt.Errorf("patient not found")
	}

	// Resolve caretaker by email when ID is not provided.
	if cmd.CaretakerID == "" {
		u, err := uc.users.FindByEmail(ctx, cmd.CaretakerEmail)
		if err != nil {
			return fmt.Errorf("no caretaker found with email %q", cmd.CaretakerEmail)
		}
		if u.Role() != user.RoleCaretaker {
			return fmt.Errorf("user with email %q is not a caretaker", cmd.CaretakerEmail)
		}
		cmd.CaretakerID = u.ID()
	}

	caretaker, err := uc.users.FindByID(ctx, cmd.CaretakerID)
	if err != nil || caretaker.Role() != user.RoleCaretaker {
		return fmt.Errorf("caretaker not found")
	}

	exists, err := uc.caretakers.Exists(ctx, cmd.PatientID, cmd.CaretakerID)
	if err != nil {
		return fmt.Errorf("check relationship: %w", err)
	}
	if exists {
		return fmt.Errorf("caretaker is already linked to this patient")
	}

	rel, err := domaincart.NewPatientCaretaker(cmd.PatientID, cmd.CaretakerID, cmd.LinkedBy)
	if err != nil {
		return err
	}

	if err := uc.caretakers.Save(ctx, rel); err != nil {
		return fmt.Errorf("persist relationship: %w", err)
	}

	uc.logger.Info("caretaker linked",
		zap.String("patientId", cmd.PatientID),
		zap.String("caretakerId", cmd.CaretakerID),
	)
	return nil
}
