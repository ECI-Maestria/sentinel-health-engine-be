// Package notifications provides a background worker that dispatches reminders and appointment notifications.
package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/sentinel-health-engine/calendar-service/internal/domain"
	"github.com/sentinel-health-engine/calendar-service/internal/postgres"
)

// EmailSender is the interface for sending email notifications.
type EmailSender interface {
	Send(ctx context.Context, recipientEmail, subject, htmlBody string) error
}

// PushNotifier is the interface for sending push notifications via FCM.
type PushNotifier interface {
	Send(ctx context.Context, fcmToken, title, body string, data map[string]string) error
}

// patientContacts is the response shape returned by the user-service internal contacts endpoint.
type patientContacts struct {
	Contacts []contactEntry `json:"contacts"`
}

type contactEntry struct {
	Email    string `json:"email"`
	FCMToken string `json:"fcmToken"`
}

// Worker polls appointment and reminder repositories and dispatches notifications.
type Worker struct {
	logger         *zap.Logger
	apptRepo       *postgres.AppointmentRepository
	reminderRepo   *postgres.ReminderRepository
	httpClient     *http.Client
	userServiceURL string
	userServiceKey string
	emailSender    EmailSender  // optional — nil means email disabled
	pushNotifier   PushNotifier // optional — nil means push disabled
}

// NewWorker creates a Worker. emailSender and pushNotifier are optional (pass nil to disable).
func NewWorker(
	logger *zap.Logger,
	apptRepo *postgres.AppointmentRepository,
	reminderRepo *postgres.ReminderRepository,
	emailSender EmailSender,
	pushNotifier PushNotifier,
) *Worker {
	return &Worker{
		logger:         logger,
		apptRepo:       apptRepo,
		reminderRepo:   reminderRepo,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		userServiceURL: os.Getenv("USER_SERVICE_URL"),
		userServiceKey: os.Getenv("USER_SERVICE_API_KEY"),
		emailSender:    emailSender,
		pushNotifier:   pushNotifier,
	}
}

// Start runs the notification loop. It blocks until ctx is cancelled.
// Call as `go worker.Start(ctx)`.
func (w *Worker) Start(ctx context.Context) {
	w.logger.Info("notification worker started")
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	// Run once immediately on start.
	w.run(ctx)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("notification worker stopped")
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

// run executes one cycle of appointment and reminder processing.
func (w *Worker) run(ctx context.Context) {
	w.processAppointments(ctx)
	w.processReminders(ctx)
}

// processAppointments sends reminders for appointments occurring within the next 60 minutes.
func (w *Worker) processAppointments(ctx context.Context) {
	now := time.Now().UTC()
	horizon := now.Add(60 * time.Minute)

	appointments, err := w.apptRepo.ListUpcoming(ctx, now, horizon)
	if err != nil {
		w.logger.Error("worker: list upcoming appointments", zap.Error(err))
		return
	}

	for _, appt := range appointments {
		contacts, err := w.fetchContacts(ctx, appt.PatientID)
		if err != nil {
			w.logger.Warn("worker: fetch contacts for appointment",
				zap.String("patientID", appt.PatientID),
				zap.Error(err),
			)
		}

		title := fmt.Sprintf("Recordatorio de cita: %s", appt.Title)
		body := fmt.Sprintf("Tienes una cita programada a las %s", appt.ScheduledAt.Local().Format("15:04"))
		if appt.Location != "" {
			body += fmt.Sprintf(" en %s", appt.Location)
		}
		htmlBody := buildAppointmentEmailHTML(appt.Title, appt.ScheduledAt, appt.Location, appt.Notes)
		data := map[string]string{
			"type":          "appointment",
			"appointmentId": appt.ID,
			"patientId":     appt.PatientID,
		}

		w.notify(ctx, contacts, title, body, htmlBody, data)

		w.logger.Info("appointment reminder dispatched",
			zap.String("patientID", appt.PatientID),
			zap.String("title", appt.Title),
			zap.Int("contacts", len(contacts)),
		)

		if err := w.apptRepo.MarkReminderSent(ctx, appt.ID); err != nil {
			w.logger.Error("worker: mark appointment reminder sent",
				zap.String("appointmentID", appt.ID),
				zap.Error(err),
			)
		}
	}
}

// processReminders sends due reminders and creates next occurrences for recurring ones.
func (w *Worker) processReminders(ctx context.Context) {
	now := time.Now().UTC()

	reminders, err := w.reminderRepo.ListDueReminders(ctx, now)
	if err != nil {
		w.logger.Error("worker: list due reminders", zap.Error(err))
		return
	}

	for _, rem := range reminders {
		contacts, err := w.fetchContacts(ctx, rem.PatientID)
		if err != nil {
			w.logger.Warn("worker: fetch contacts for reminder",
				zap.String("patientID", rem.PatientID),
				zap.Error(err),
			)
		}

		title := rem.Title
		body := rem.Message
		htmlBody := buildReminderEmailHTML(rem.Title, rem.Message, rem.ReminderAt)
		data := map[string]string{
			"type":       "reminder",
			"reminderId": rem.ID,
			"patientId":  rem.PatientID,
		}

		w.notify(ctx, contacts, title, body, htmlBody, data)

		w.logger.Info("reminder dispatched",
			zap.String("patientID", rem.PatientID),
			zap.String("title", rem.Title),
			zap.Int("contacts", len(contacts)),
		)

		if err := w.reminderRepo.MarkSent(ctx, rem.ID); err != nil {
			w.logger.Error("worker: mark reminder sent",
				zap.String("reminderID", rem.ID),
				zap.Error(err),
			)
			continue
		}

		// For recurring reminders, create the next occurrence.
		if rem.Recurrence != domain.RecurrenceNone {
			next, err := nextOccurrence(rem)
			if err != nil {
				w.logger.Warn("worker: compute next occurrence",
					zap.String("reminderID", rem.ID),
					zap.Error(err),
				)
				continue
			}
			if err := w.reminderRepo.Save(ctx, next); err != nil {
				w.logger.Error("worker: save next reminder occurrence",
					zap.String("reminderID", rem.ID),
					zap.Error(err),
				)
			}
		}
	}
}

// notify sends email and push notifications to all contacts.
func (w *Worker) notify(ctx context.Context, contacts []contactEntry, title, pushBody, htmlBody string, data map[string]string) {
	for _, contact := range contacts {
		// Push notification
		if w.pushNotifier != nil && contact.FCMToken != "" {
			if err := w.pushNotifier.Send(ctx, contact.FCMToken, title, pushBody, data); err != nil {
				w.logger.Warn("worker: push notification failed",
					zap.String("title", title),
					zap.Error(err),
				)
			}
		}

		// Email notification
		if w.emailSender != nil && contact.Email != "" {
			if err := w.emailSender.Send(ctx, contact.Email, title, htmlBody); err != nil {
				w.logger.Warn("worker: email notification failed",
					zap.String("to", contact.Email),
					zap.Error(err),
				)
			}
		}
	}
}

// nextOccurrence computes the next Reminder occurrence for a recurring reminder.
func nextOccurrence(rem *domain.Reminder) (*domain.Reminder, error) {
	var next time.Time
	switch rem.Recurrence {
	case domain.RecurrenceDaily:
		next = rem.ReminderAt.AddDate(0, 0, 1)
	case domain.RecurrenceWeekly:
		next = rem.ReminderAt.AddDate(0, 0, 7)
	case domain.RecurrenceMonthly:
		next = rem.ReminderAt.AddDate(0, 1, 0)
	default:
		return nil, fmt.Errorf("unsupported recurrence: %s", rem.Recurrence)
	}

	return domain.NewReminder(
		rem.PatientID,
		rem.CreatedBy,
		rem.Title,
		rem.Message,
		next,
		rem.Recurrence,
	)
}

// fetchContacts retrieves the contact list for a patient from the user-service internal API.
func (w *Worker) fetchContacts(ctx context.Context, patientID string) ([]contactEntry, error) {
	if w.userServiceURL == "" {
		return nil, fmt.Errorf("USER_SERVICE_URL is not configured")
	}

	url := fmt.Sprintf("%s/v1/internal/patients/%s/contacts", w.userServiceURL, patientID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("X-Internal-API-Key", w.userServiceKey)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user-service returned status %d", resp.StatusCode)
	}

	var result patientContacts
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Contacts, nil
}

// ── Email HTML builders ──────────────────────────────────────────────────────

func buildAppointmentEmailHTML(title string, scheduledAt time.Time, location, notes string) string {
	loc := ""
	if location != "" {
		loc = fmt.Sprintf("<p><strong>Lugar:</strong> %s</p>", location)
	}
	notesHTML := ""
	if notes != "" {
		notesHTML = fmt.Sprintf("<p><strong>Notas:</strong> %s</p>", notes)
	}
	return fmt.Sprintf(`
<html><body style="font-family:sans-serif;max-width:600px;margin:0 auto;padding:20px;">
  <div style="background:#1a202c;color:#e2e8f0;padding:16px 24px;border-radius:8px 8px 0 0;">
    <h2 style="margin:0;">🗓️ Recordatorio de Cita</h2>
  </div>
  <div style="border:1px solid #e2e8f0;border-top:none;padding:24px;border-radius:0 0 8px 8px;">
    <h3 style="color:#2d3748;">%s</h3>
    <p><strong>Fecha y hora:</strong> %s</p>
    %s
    %s
    <hr style="border:none;border-top:1px solid #e2e8f0;margin:20px 0;">
    <p style="color:#718096;font-size:12px;">Sentinel Health Engine — Sistema de Monitoreo de Salud</p>
  </div>
</body></html>`,
		title,
		scheduledAt.Local().Format("02/01/2006 15:04"),
		loc,
		notesHTML,
	)
}

func buildReminderEmailHTML(title, message string, reminderAt time.Time) string {
	return fmt.Sprintf(`
<html><body style="font-family:sans-serif;max-width:600px;margin:0 auto;padding:20px;">
  <div style="background:#1a202c;color:#e2e8f0;padding:16px 24px;border-radius:8px 8px 0 0;">
    <h2 style="margin:0;">🔔 Recordatorio</h2>
  </div>
  <div style="border:1px solid #e2e8f0;border-top:none;padding:24px;border-radius:0 0 8px 8px;">
    <h3 style="color:#2d3748;">%s</h3>
    <p>%s</p>
    <p><strong>Fecha:</strong> %s</p>
    <hr style="border:none;border-top:1px solid #e2e8f0;margin:20px 0;">
    <p style="color:#718096;font-size:12px;">Sentinel Health Engine — Sistema de Monitoreo de Salud</p>
  </div>
</body></html>`,
		title,
		message,
		reminderAt.Local().Format("02/01/2006 15:04"),
	)
}
