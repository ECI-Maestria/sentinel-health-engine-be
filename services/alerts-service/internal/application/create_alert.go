// Package application contains the use cases for the Alerts bounded context.
package application

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	sharedevents "github.com/sentinel-health-engine/shared/events"
	"github.com/sentinel-health-engine/alerts-service/internal/domain"
)

// PushNotifier is the port for push notification delivery.
type PushNotifier interface {
	SendAlert(ctx context.Context, fcmToken, title, body string, data map[string]string) error
}

// EmailSender is the port for email delivery.
type EmailSender interface {
	SendAlert(ctx context.Context, recipientEmail, subject, htmlBody string) error
}

// PatientContact holds notification contact info for a patient.
type PatientContact struct {
	PatientID string
	FCMToken  string
	Email     string
}

// ContactResolver resolves notification contacts for a patient at runtime.
// Implemented by the user-service HTTP client.
type ContactResolver interface {
	GetContacts(ctx context.Context, patientID string) ([]PatientContact, error)
}

// CreateAlertUseCase handles the full alert lifecycle:
// create → persist → send push → send email → mark sent.
type CreateAlertUseCase struct {
	repository      domain.AlertRepository
	pushNotifier    PushNotifier
	emailSender     EmailSender
	contactResolver ContactResolver
	logger          *zap.Logger
}

func NewCreateAlertUseCase(
	repo domain.AlertRepository,
	push PushNotifier,
	email EmailSender,
	resolver ContactResolver,
	logger *zap.Logger,
) *CreateAlertUseCase {
	return &CreateAlertUseCase{
		repository:      repo,
		pushNotifier:    push,
		emailSender:     email,
		contactResolver: resolver,
		logger:          logger,
	}
}

// Execute processes an AnomalyDetectedEvent end-to-end.
func (uc *CreateAlertUseCase) Execute(ctx context.Context, event sharedevents.AnomalyDetectedEvent) error {
	log := uc.logger.With(
		zap.String("patientId", event.PatientID),
		zap.String("severity", string(event.MaxSeverity)),
	)

	// 1. Create alert aggregate
	alert, err := domain.NewAlert(event)
	if err != nil {
		return fmt.Errorf("build alert: %w", err)
	}

	// 2. Persist
	if err := uc.repository.Save(ctx, alert); err != nil {
		return fmt.Errorf("persist alert: %w", err)
	}
	log.Info("alert persisted", zap.String("alertId", alert.ID()))

	// 3. Resolve contacts from user-service
	contacts, err := uc.contactResolver.GetContacts(ctx, event.PatientID)
	if err != nil {
		log.Warn("could not resolve contacts — alert persisted but notifications skipped",
			zap.Error(err))
		return nil
	}
	if len(contacts) == 0 {
		log.Warn("no contacts found for patient — notifications skipped")
		return nil
	}

	// 4. Compose notification content
	title := "⚠️ Alerta de Signos Vitales"
	if event.MaxSeverity == sharedevents.SeverityCritical {
		title = "🚨 ALERTA CRÍTICA — Signos Vitales"
	}

	// 5. Notify all contacts
	for _, contact := range contacts {
		if uc.pushNotifier != nil && contact.FCMToken != "" {
			pushData := map[string]string{
				"alertId":   alert.ID(),
				"patientId": event.PatientID,
				"severity":  string(event.MaxSeverity),
			}
			if err := uc.pushNotifier.SendAlert(ctx, contact.FCMToken, title, alert.Message(), pushData); err != nil {
				log.Error("push notification failed — continuing",
					zap.String("email", contact.Email), zap.Error(err))
			}
		}

		if uc.emailSender != nil && contact.Email != "" {
			subject := fmt.Sprintf("[Sentinel Health] %s — Paciente %s", title, event.PatientID)
			html := buildEmailHTML(alert, event)
			if err := uc.emailSender.SendAlert(ctx, contact.Email, subject, html); err != nil {
				log.Error("email notification failed — continuing",
					zap.String("email", contact.Email), zap.Error(err))
			}
		}
	}

	// 6. Mark as sent
	alert.MarkSent()
	if err := uc.repository.Update(ctx, alert); err != nil {
		log.Warn("failed to update alert status to SENT", zap.Error(err))
	}

	log.Info("alert dispatched", zap.String("alertId", alert.ID()))
	return nil
}

func buildEmailHTML(alert *domain.Alert, event sharedevents.AnomalyDetectedEvent) string {
	color := "#f59e0b"
	if event.MaxSeverity == sharedevents.SeverityCritical {
		color = "#dc2626"
	}

	rows := ""
	for _, v := range event.Violations {
		rows += fmt.Sprintf("<li><strong>%s</strong>: %.1f (umbral: %.1f)</li>",
			v.RuleName, v.ActualValue, v.Threshold)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html><body style="font-family:Arial,sans-serif;max-width:600px;margin:0 auto;">
  <div style="background:%s;color:white;padding:20px;border-radius:8px 8px 0 0;">
    <h1 style="margin:0;font-size:22px;">%s</h1>
    <p style="margin:4px 0 0;">Sentinel Health Engine</p>
  </div>
  <div style="background:#f9f9f9;padding:20px;border-radius:0 0 8px 8px;">
    <p><strong>Paciente ID:</strong> %s</p>
    <p><strong>Heart Rate:</strong> %d bpm</p>
    <p><strong>SpO2:</strong> %.1f%%</p>
    <hr/>
    <p><strong>Reglas violadas:</strong></p>
    <ul>%s</ul>
    <hr/>
    <p style="color:#888;font-size:11px;">Alert ID: %s | Sentinel Health Engine</p>
  </div>
</body></html>`,
		color, alert.Message(),
		event.PatientID, event.HeartRate, event.SpO2,
		rows, alert.ID(),
	)
}
