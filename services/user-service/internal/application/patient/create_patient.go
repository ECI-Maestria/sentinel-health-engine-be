// Package patient contains use cases for patient management (Doctor-only operations).
package patient

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"github.com/sentinel-health-engine/user-service/internal/domain/user"
)

const passwordChars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// WelcomeEmailSender is the port for sending welcome emails with temporary passwords.
type WelcomeEmailSender interface {
	SendWelcome(ctx context.Context, email, fullName, temporaryPassword string) error
}

// CreatePatientCommand is the input DTO.
type CreatePatientCommand struct {
	FirstName string
	LastName  string
	Email     string
}

// CreatePatientResult contains the created patient and the plain-text temporary password.
type CreatePatientResult struct {
	Patient           *user.User
	TemporaryPassword string
}

// CreatePatientUseCase registers a new patient (Doctor-only).
// Generates a temporary password, creates the user, and sends a welcome email.
type CreatePatientUseCase struct {
	users        user.Repository
	emailSender  WelcomeEmailSender
	logger       *zap.Logger
}

func NewCreatePatientUseCase(users user.Repository, emailSender WelcomeEmailSender, logger *zap.Logger) *CreatePatientUseCase {
	return &CreatePatientUseCase{users: users, emailSender: emailSender, logger: logger}
}

// Execute creates the patient, persists it, and sends the welcome email.
func (uc *CreatePatientUseCase) Execute(ctx context.Context, cmd CreatePatientCommand) (*CreatePatientResult, error) {
	exists, err := uc.users.ExistsByEmail(ctx, cmd.Email)
	if err != nil {
		return nil, fmt.Errorf("check email uniqueness: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("a user with email %q already exists", cmd.Email)
	}

	password, err := GeneratePassword(12)
	if err != nil {
		return nil, fmt.Errorf("generate password: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u, err := user.NewUser(uuid.NewString(), cmd.Email, string(hash), cmd.FirstName, cmd.LastName, user.RolePatient)
	if err != nil {
		return nil, fmt.Errorf("create patient: %w", err)
	}

	if err := uc.users.Save(ctx, u); err != nil {
		return nil, fmt.Errorf("persist patient: %w", err)
	}

	if err := uc.emailSender.SendWelcome(ctx, u.Email(), u.FullName(), password); err != nil {
		uc.logger.Error("welcome email failed — patient created but email not sent",
			zap.String("patientId", u.ID()), zap.Error(err))
	}

	uc.logger.Info("patient created",
		zap.String("patientId", u.ID()),
		zap.String("email", u.Email()),
	)
	return &CreatePatientResult{Patient: u, TemporaryPassword: password}, nil
}

// GeneratePassword creates a cryptographically random alphanumeric password of the given length.
func GeneratePassword(length int) (string, error) {
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordChars))))
		if err != nil {
			return "", err
		}
		result[i] = passwordChars[n.Int64()]
	}
	return string(result), nil
}
