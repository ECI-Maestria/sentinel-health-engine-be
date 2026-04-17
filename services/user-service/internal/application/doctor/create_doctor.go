// Package doctor contains use cases for doctor management.
package doctor

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/sentinel-health-engine/user-service/internal/application/patient"
	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

// WelcomeEmailSender is the port for sending welcome emails with temporary passwords.
type WelcomeEmailSender interface {
	SendWelcome(ctx context.Context, email, fullName, temporaryPassword string) error
}

// CreateDoctorCommand is the input DTO.
type CreateDoctorCommand struct {
	FirstName string
	LastName  string
	Email     string
}

// CreateDoctorResult contains the created doctor and the plain-text temporary password.
type CreateDoctorResult struct {
	Doctor            *user.User
	TemporaryPassword string
}

// CreateDoctorUseCase registers a new doctor (Doctor-only operation).
// Generates a temporary password, creates the user with DOCTOR role, and sends a welcome email.
type CreateDoctorUseCase struct {
	users       user.Repository
	emailSender WelcomeEmailSender
	logger      *zap.Logger
}

func NewCreateDoctorUseCase(users user.Repository, emailSender WelcomeEmailSender, logger *zap.Logger) *CreateDoctorUseCase {
	return &CreateDoctorUseCase{users: users, emailSender: emailSender, logger: logger}
}

// Execute creates the doctor, persists it, and sends the welcome email.
func (uc *CreateDoctorUseCase) Execute(ctx context.Context, cmd CreateDoctorCommand) (*CreateDoctorResult, error) {
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

	u, err := user.NewUser(uuid.NewString(), cmd.Email, string(hash), cmd.FirstName, cmd.LastName, user.RoleDoctor)
	if err != nil {
		return nil, fmt.Errorf("create doctor: %w", err)
	}

	if err := uc.users.Save(ctx, u); err != nil {
		return nil, fmt.Errorf("persist doctor: %w", err)
	}

	if err := uc.emailSender.SendWelcome(ctx, u.Email(), u.FullName(), password); err != nil {
		uc.logger.Error("welcome email failed — doctor created but email not sent",
			zap.String("doctorId", u.ID()), zap.Error(err))
	}

	uc.logger.Info("doctor created",
		zap.String("doctorId", u.ID()),
		zap.String("email", u.Email()),
	)
	return &CreateDoctorResult{Doctor: u, TemporaryPassword: password}, nil
}
