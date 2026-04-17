package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sentinel-health-engine/calendar-service/internal/domain"
)

// MedicationRepository handles persistence for Medication domain objects.
type MedicationRepository struct {
	pool *pgxpool.Pool
}

// NewMedicationRepository creates a new MedicationRepository.
func NewMedicationRepository(pool *pgxpool.Pool) *MedicationRepository {
	return &MedicationRepository{pool: pool}
}

// Save inserts a new Medication into the database.
func (r *MedicationRepository) Save(ctx context.Context, m *domain.Medication) error {
	timesJSON, err := json.Marshal(m.ScheduledTimes)
	if err != nil {
		return fmt.Errorf("marshal scheduled_times: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO medications
			(id, patient_id, prescribed_by, name, dosage, frequency, scheduled_times, start_date, end_date, notes, is_active, created_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`,
		m.ID, m.PatientID, m.PrescribedBy, m.Name, m.Dosage,
		string(m.Frequency), timesJSON, m.StartDate, m.EndDate,
		m.Notes, m.IsActive, m.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert medication: %w", err)
	}
	return nil
}

// FindByID retrieves a single Medication by its UUID string.
func (r *MedicationRepository) FindByID(ctx context.Context, id string) (*domain.Medication, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, patient_id, prescribed_by, name, dosage, frequency,
		       scheduled_times, start_date, end_date, notes, is_active, created_at
		FROM medications
		WHERE id = $1
	`, id)

	m, err := scanMedication(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("find medication by id: %w", err)
	}
	return m, nil
}

// ListByPatient returns all medications for a patient. If activeOnly is true, only active ones are returned.
func (r *MedicationRepository) ListByPatient(ctx context.Context, patientID string, activeOnly bool) ([]*domain.Medication, error) {
	query := `
		SELECT id, patient_id, prescribed_by, name, dosage, frequency,
		       scheduled_times, start_date, end_date, notes, is_active, created_at
		FROM medications
		WHERE patient_id = $1
	`
	args := []interface{}{patientID}

	if activeOnly {
		query += " AND is_active = true"
	}
	query += " ORDER BY created_at ASC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list medications by patient: %w", err)
	}
	defer rows.Close()

	return collectMedications(rows)
}

// Update persists changes to an existing Medication record.
func (r *MedicationRepository) Update(ctx context.Context, m *domain.Medication) error {
	timesJSON, err := json.Marshal(m.ScheduledTimes)
	if err != nil {
		return fmt.Errorf("marshal scheduled_times: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		UPDATE medications
		SET name = $2, dosage = $3, frequency = $4, scheduled_times = $5,
		    start_date = $6, end_date = $7, notes = $8, is_active = $9
		WHERE id = $1
	`,
		m.ID, m.Name, m.Dosage, string(m.Frequency), timesJSON,
		m.StartDate, m.EndDate, m.Notes, m.IsActive,
	)
	if err != nil {
		return fmt.Errorf("update medication: %w", err)
	}
	return nil
}

// Deactivate sets is_active=false for the given medication (soft delete).
func (r *MedicationRepository) Deactivate(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE medications SET is_active = false WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deactivate medication: %w", err)
	}
	return nil
}

// scanMedication scans a single pgx Row into a Medication domain object.
func scanMedication(row pgx.Row) (*domain.Medication, error) {
	var m domain.Medication
	var freq string
	var timesRaw []byte
	var startDate time.Time
	var endDate *time.Time

	err := row.Scan(
		&m.ID, &m.PatientID, &m.PrescribedBy, &m.Name, &m.Dosage, &freq,
		&timesRaw, &startDate, &endDate, &m.Notes, &m.IsActive, &m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	m.Frequency = domain.Frequency(freq)
	m.StartDate = startDate
	m.EndDate = endDate

	var times []string
	if err := json.Unmarshal(timesRaw, &times); err != nil {
		return nil, fmt.Errorf("unmarshal scheduled_times: %w", err)
	}
	if times == nil {
		times = []string{}
	}
	m.ScheduledTimes = times

	return &m, nil
}

// collectMedications iterates pgx.Rows and builds a slice of Medication pointers.
func collectMedications(rows pgx.Rows) ([]*domain.Medication, error) {
	var result []*domain.Medication
	for rows.Next() {
		var m domain.Medication
		var freq string
		var timesRaw []byte
		var startDate time.Time
		var endDate *time.Time

		err := rows.Scan(
			&m.ID, &m.PatientID, &m.PrescribedBy, &m.Name, &m.Dosage, &freq,
			&timesRaw, &startDate, &endDate, &m.Notes, &m.IsActive, &m.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan medication row: %w", err)
		}

		m.Frequency = domain.Frequency(freq)
		m.StartDate = startDate
		m.EndDate = endDate

		var times []string
		if err := json.Unmarshal(timesRaw, &times); err != nil {
			return nil, fmt.Errorf("unmarshal scheduled_times: %w", err)
		}
		if times == nil {
			times = []string{}
		}
		m.ScheduledTimes = times

		result = append(result, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate medication rows: %w", err)
	}
	return result, nil
}
