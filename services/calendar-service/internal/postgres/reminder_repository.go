package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sentinel-health-engine/calendar-service/internal/domain"
)

// ReminderRepository handles persistence for Reminder domain objects.
type ReminderRepository struct {
	pool *pgxpool.Pool
}

// NewReminderRepository creates a new ReminderRepository.
func NewReminderRepository(pool *pgxpool.Pool) *ReminderRepository {
	return &ReminderRepository{pool: pool}
}

// Save inserts a new Reminder into the database.
func (r *ReminderRepository) Save(ctx context.Context, rem *domain.Reminder) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO reminders
			(id, patient_id, created_by, title, message, reminder_at, recurrence, status, sent_at, created_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		rem.ID, rem.PatientID, rem.CreatedBy, rem.Title, rem.Message,
		rem.ReminderAt, string(rem.Recurrence), string(rem.Status),
		rem.SentAt, rem.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert reminder: %w", err)
	}
	return nil
}

// FindByID retrieves a single Reminder by its UUID string.
func (r *ReminderRepository) FindByID(ctx context.Context, id string) (*domain.Reminder, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, patient_id, created_by, title, message, reminder_at, recurrence, status, sent_at, created_at
		FROM reminders
		WHERE id = $1
	`, id)

	rem, err := scanReminder(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find reminder by id: %w", err)
	}
	return rem, nil
}

// ListByPatient returns all reminders for a patient ordered by reminder_at ASC.
func (r *ReminderRepository) ListByPatient(ctx context.Context, patientID string) ([]*domain.Reminder, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, patient_id, created_by, title, message, reminder_at, recurrence, status, sent_at, created_at
		FROM reminders
		WHERE patient_id = $1
		ORDER BY reminder_at ASC
	`, patientID)
	if err != nil {
		return nil, fmt.Errorf("list reminders by patient: %w", err)
	}
	defer rows.Close()

	return collectReminders(rows)
}

// ListDueReminders returns all PENDING reminders with reminder_at <= before.
// Used by the notification worker.
func (r *ReminderRepository) ListDueReminders(ctx context.Context, before time.Time) ([]*domain.Reminder, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, patient_id, created_by, title, message, reminder_at, recurrence, status, sent_at, created_at
		FROM reminders
		WHERE status = 'PENDING'
		  AND reminder_at <= $1
		ORDER BY reminder_at ASC
	`, before)
	if err != nil {
		return nil, fmt.Errorf("list due reminders: %w", err)
	}
	defer rows.Close()

	return collectReminders(rows)
}

// MarkSent sets status=SENT and sent_at=now for the given reminder.
func (r *ReminderRepository) MarkSent(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE reminders
		SET status = 'SENT', sent_at = $2
		WHERE id = $1
	`, id, now)
	if err != nil {
		return fmt.Errorf("mark reminder sent: %w", err)
	}
	return nil
}

// Update persists changes to an existing Reminder record.
func (r *ReminderRepository) Update(ctx context.Context, rem *domain.Reminder) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE reminders
		SET title = $2, message = $3, reminder_at = $4, recurrence = $5, status = $6, sent_at = $7
		WHERE id = $1
	`,
		rem.ID, rem.Title, rem.Message, rem.ReminderAt,
		string(rem.Recurrence), string(rem.Status), rem.SentAt,
	)
	if err != nil {
		return fmt.Errorf("update reminder: %w", err)
	}
	return nil
}

// Delete permanently removes a Reminder from the database.
func (r *ReminderRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM reminders WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete reminder: %w", err)
	}
	return nil
}

// scanReminder scans a single pgx Row into a Reminder domain object.
func scanReminder(row pgx.Row) (*domain.Reminder, error) {
	var rem domain.Reminder
	var recurrence, status string
	err := row.Scan(
		&rem.ID, &rem.PatientID, &rem.CreatedBy, &rem.Title, &rem.Message,
		&rem.ReminderAt, &recurrence, &status, &rem.SentAt, &rem.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	rem.Recurrence = domain.Recurrence(recurrence)
	rem.Status = domain.ReminderStatus(status)
	return &rem, nil
}

// collectReminders iterates pgx.Rows and builds a slice of Reminder pointers.
func collectReminders(rows pgx.Rows) ([]*domain.Reminder, error) {
	var result []*domain.Reminder
	for rows.Next() {
		var rem domain.Reminder
		var recurrence, status string
		err := rows.Scan(
			&rem.ID, &rem.PatientID, &rem.CreatedBy, &rem.Title, &rem.Message,
			&rem.ReminderAt, &recurrence, &status, &rem.SentAt, &rem.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan reminder row: %w", err)
		}
		rem.Recurrence = domain.Recurrence(recurrence)
		rem.Status = domain.ReminderStatus(status)
		result = append(result, &rem)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reminder rows: %w", err)
	}
	return result, nil
}
