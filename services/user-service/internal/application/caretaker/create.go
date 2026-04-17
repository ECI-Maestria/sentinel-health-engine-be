package caretaker

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/sentinel-health-engine/user-service/internal/application/patient"
	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// CreateCaretakerCommand is the input DTO.
type CreateCaretakerCommand struct {
	FirstName string
	LastName  string
	Email     string
}

// CreateCaretakerUseCase registers a new caretaker (Doctor-only).
// Generates a temporary password and sends a welcome email.
type CreateCaretakerUseCase struct {
	users       user.Repository
	emailSender patient.WelcomeEmailSender
	logger      *zap.Logger
}

func NewCreateCaretakerUseCase(users user.Repository, emailSender patient.WelcomeEmailSender, logger *zap.Logger) *CreateCaretakerUseCase {
	return &CreateCaretakerUseCase{users: users, emailSender: emailSender, logger: logger}
}

// Execute creates the caretaker account, persists it, and sends a welcome email.
func (uc *CreateCaretakerUseCase) Execute(ctx context.Context, cmd CreateCaretakerCommand) (*user.User, error) {
	exists, err := uc.users.ExistsByEmail(ctx, cmd.Email)
	if err != nil {
		return nil, fmt.Errorf("check email uniqueness: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("a user with email %q already exists", cmd.Email)
	}

	password, err := patient.GeneratePassword(12)
	if err != nil {
		return nil, fmt.Errorf("generate password: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u, err := user.NewUser(uuid.NewString(), cmd.Email, string(hash), cmd.FirstName, cmd.LastName, user.RoleCaretaker)
	if err != nil {
		return nil, fmt.Errorf("create caretaker: %w", err)
	}

	if err := uc.users.Save(ctx, u); err != nil {
		return nil, fmt.Errorf("persist caretaker: %w", err)
	}

	if err := uc.emailSender.SendWelcome(ctx, u.Email(), u.FullName(), password); err != nil {
		uc.logger.Error("welcome email failed — caretaker created but email not sent",
			zap.String("caretakerId", u.ID()), zap.Error(err))
	}

	uc.logger.Info("caretaker created",
		zap.String("caretakerId", u.ID()),
		zap.String("email", u.Email()),
	)
	return u, nil
}
