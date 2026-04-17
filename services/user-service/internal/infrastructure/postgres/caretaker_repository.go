package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	domaincart "github.com/sentinel-health-engine/user-service/internal/domain/caretaker"
)

// CaretakerRepository implements caretaker.Repository backed by PostgreSQL.
type CaretakerRepository struct {
	pool *pgxpool.Pool
}

func NewCaretakerRepository(pool *pgxpool.Pool) *CaretakerRepository {
	return &CaretakerRepository{pool: pool}
}

func (r *CaretakerRepository) Save(ctx context.Context, pc *domaincart.PatientCaretaker) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO patient_caretakers (patient_id, caretaker_id, linked_by, created_at)
		VALUES ($1, $2, $3, $4)
	`, pc.PatientID(), pc.CaretakerID(), pc.LinkedBy(), pc.CreatedAt())
	if err != nil {
		return fmt.Errorf("insert patient_caretaker: %w", err)
	}
	return nil
}

func (r *CaretakerRepository) Delete(ctx context.Context, patientID, caretakerID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM patient_caretakers WHERE patient_id = $1 AND caretaker_id = $2
	`, patientID, caretakerID)
	if err != nil {
		return fmt.Errorf("delete patient_caretaker: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("relationship not found")
	}
	return nil
}

func (r *CaretakerRepository) ListCaretakersByPatient(ctx context.Context, patientID string) ([]*domaincart.PatientCaretaker, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT patient_id, caretaker_id, linked_by, created_at
		FROM patient_caretakers WHERE patient_id = $1 ORDER BY created_at ASC
	`, patientID)
	if err != nil {
		return nil, fmt.Errorf("list caretakers: %w", err)
	}
	defer rows.Close()
	return scanRelationships(rows)
}

func (r *CaretakerRepository) ListPatientsByCaretaker(ctx context.Context, caretakerID string) ([]*domaincart.PatientCaretaker, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT patient_id, caretaker_id, linked_by, created_at
		FROM patient_caretakers WHERE caretaker_id = $1 ORDER BY created_at ASC
	`, caretakerID)
	if err != nil {
		return nil, fmt.Errorf("list patients: %w", err)
	}
	defer rows.Close()
	return scanRelationships(rows)
}

func (r *CaretakerRepository) Exists(ctx context.Context, patientID, caretakerID string) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM patient_caretakers WHERE patient_id = $1 AND caretaker_id = $2
	`, patientID, caretakerID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check relationship: %w", err)
	}
	return count > 0, nil
}

func scanRelationships(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]*domaincart.PatientCaretaker, error) {
	var result []*domaincart.PatientCaretaker
	for rows.Next() {
		var (
			patientID   string
			caretakerID string
			linkedBy    string
			createdAt   time.Time
		)
		if err := rows.Scan(&patientID, &caretakerID, &linkedBy, &createdAt); err != nil {
			return nil, fmt.Errorf("scan relationship: %w", err)
		}
		result = append(result, domaincart.Reconstitute(patientID, caretakerID, linkedBy, createdAt))
	}
	return result, rows.Err()
}
