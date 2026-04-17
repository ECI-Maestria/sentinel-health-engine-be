package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sentinel-health-engine/calendar-service/internal/domain"
)

// AppointmentRepository handles persistence for Appointment domain objects.
type AppointmentRepository struct {
	pool *pgxpool.Pool
}

// NewAppointmentRepository creates a new AppointmentRepository.
func NewAppointmentRepository(pool *pgxpool.Pool) *AppointmentRepository {
	return &AppointmentRepository{pool: pool}
}

// Save inserts a new Appointment into the database.
func (r *AppointmentRepository) Save(ctx context.Context, a *domain.Appointment) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO appointments
			(id, patient_id, doctor_id, title, scheduled_at, location, notes, status, reminder_sent_at, created_at, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		a.ID, a.PatientID, a.DoctorID, a.Title, a.ScheduledAt,
		a.Location, a.Notes, string(a.Status), a.ReminderSentAt,
		a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert appointment: %w", err)
	}
	return nil
}

// FindByID retrieves a single Appointment by its UUID string.
func (r *AppointmentRepository) FindByID(ctx context.Context, id string) (*domain.Appointment, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, patient_id, doctor_id, title, scheduled_at, location, notes, status,
		       reminder_sent_at, created_at, updated_at
		FROM appointments
		WHERE id = $1
	`, id)

	a, err := scanAppointment(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find appointment by id: %w", err)
	}
	return a, nil
}

// ListByPatient returns all appointments for a patient ordered by scheduled_at ASC.
func (r *AppointmentRepository) ListByPatient(ctx context.Context, patientID string) ([]*domain.Appointment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, patient_id, doctor_id, title, scheduled_at, location, notes, status,
		       reminder_sent_at, created_at, updated_at
		FROM appointments
		WHERE patient_id = $1
		ORDER BY scheduled_at ASC
	`, patientID)
	if err != nil {
		return nil, fmt.Errorf("list appointments by patient: %w", err)
	}
	defer rows.Close()

	return collectAppointments(rows)
}

// ListUpcoming returns all SCHEDULED appointments within [from, to] that have no reminder sent yet.
// Used by the notification worker.
func (r *AppointmentRepository) ListUpcoming(ctx context.Context, from, to time.Time) ([]*domain.Appointment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, patient_id, doctor_id, title, scheduled_at, location, notes, status,
		       reminder_sent_at, created_at, updated_at
		FROM appointments
		WHERE status = 'SCHEDULED'
		  AND reminder_sent_at IS NULL
		  AND scheduled_at >= $1
		  AND scheduled_at <= $2
		ORDER BY scheduled_at ASC
	`, from, to)
	if err != nil {
		return nil, fmt.Errorf("list upcoming appointments: %w", err)
	}
	defer rows.Close()

	return collectAppointments(rows)
}

// Update persists changes to an existing Appointment record.
func (r *AppointmentRepository) Update(ctx context.Context, a *domain.Appointment) error {
	a.UpdatedAt = time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE appointments
		SET title = $2, scheduled_at = $3, location = $4, notes = $5,
		    status = $6, reminder_sent_at = $7, updated_at = $8
		WHERE id = $1
	`,
		a.ID, a.Title, a.ScheduledAt, a.Location, a.Notes,
		string(a.Status), a.ReminderSentAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update appointment: %w", err)
	}
	return nil
}

// MarkReminderSent sets reminder_sent_at to now for the given appointment.
func (r *AppointmentRepository) MarkReminderSent(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE appointments
		SET reminder_sent_at = $2, updated_at = $3
		WHERE id = $1
	`, id, now, now)
	if err != nil {
		return fmt.Errorf("mark reminder sent: %w", err)
	}
	return nil
}

// Delete permanently removes an Appointment from the database.
func (r *AppointmentRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM appointments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete appointment: %w", err)
	}
	return nil
}

// scanAppointment scans a single pgx Row into an Appointment domain object.
func scanAppointment(row pgx.Row) (*domain.Appointment, error) {
	var a domain.Appointment
	var status string
	err := row.Scan(
		&a.ID, &a.PatientID, &a.DoctorID, &a.Title, &a.ScheduledAt,
		&a.Location, &a.Notes, &status, &a.ReminderSentAt,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	a.Status = domain.AppointmentStatus(status)
	return &a, nil
}

// collectAppointments iterates pgx.Rows and builds a slice of Appointment pointers.
func collectAppointments(rows pgx.Rows) ([]*domain.Appointment, error) {
	var result []*domain.Appointment
	for rows.Next() {
		var a domain.Appointment
		var status string
		err := rows.Scan(
			&a.ID, &a.PatientID, &a.DoctorID, &a.Title, &a.ScheduledAt,
			&a.Location, &a.Notes, &status, &a.ReminderSentAt,
			&a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan appointment row: %w", err)
		}
		a.Status = domain.AppointmentStatus(status)
		result = append(result, &a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate appointment rows: %w", err)
	}
	return result, nil
}
