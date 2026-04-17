package caretaker

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	domaincart "github.com/sentinel-health-engine/user-service/internal/domain/caretaker"
)

// UnlinkCaretakerCommand is the input DTO.
type UnlinkCaretakerCommand struct {
	PatientID   string
	CaretakerID string
}

// UnlinkCaretakerUseCase removes a patient–caretaker relationship.
type UnlinkCaretakerUseCase struct {
	caretakers domaincart.Repository
	logger     *zap.Logger
}

func NewUnlinkCaretakerUseCase(caretakers domaincart.Repository, logger *zap.Logger) *UnlinkCaretakerUseCase {
	return &UnlinkCaretakerUseCase{caretakers: caretakers, logger: logger}
}

func (uc *UnlinkCaretakerUseCase) Execute(ctx context.Context, cmd UnlinkCaretakerCommand) error {
	exists, err := uc.caretakers.Exists(ctx, cmd.PatientID, cmd.CaretakerID)
	if err != nil {
		return fmt.Errorf("check relationship: %w", err)
	}
	if !exists {
		return fmt.Errorf("relationship not found")
	}

	if err := uc.caretakers.Delete(ctx, cmd.PatientID, cmd.CaretakerID); err != nil {
		return fmt.Errorf("delete relationship: %w", err)
	}

	uc.logger.Info("caretaker unlinked",
		zap.String("patientId", cmd.PatientID),
		zap.String("caretakerId", cmd.CaretakerID),
	)
	return nil
}
