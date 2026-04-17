package caretaker

import "context"

// Repository is the port for PatientCaretaker relationship persistence.
type Repository interface {
	Save(ctx context.Context, pc *PatientCaretaker) error
	Delete(ctx context.Context, patientID, caretakerID string) error
	ListCaretakersByPatient(ctx context.Context, patientID string) ([]*PatientCaretaker, error)
	ListPatientsByCaretaker(ctx context.Context, caretakerID string) ([]*PatientCaretaker, error)
	Exists(ctx context.Context, patientID, caretakerID string) (bool, error)
}
