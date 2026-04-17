package application_test

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"

	sharedevents "github.com/sentinel-health-engine/shared/events"
	"github.com/sentinel-health-engine/alerts-service/internal/application"
	"github.com/sentinel-health-engine/alerts-service/internal/domain"
)

// --- Mocks ---

type mockAlertRepository struct {
	saveErr   error
	updateErr error
	savedID   string
}

func (m *mockAlertRepository) Save(_ context.Context, a *domain.Alert) error {
	m.savedID = a.ID()
	return m.saveErr
}

func (m *mockAlertRepository) Update(_ context.Context, _ *domain.Alert) error {
	return m.updateErr
}

type mockPushNotifier struct {
	err    error
	called bool
}

func (m *mockPushNotifier) SendAlert(_ context.Context, _, _, _ string, _ map[string]string) error {
	m.called = true
	return m.err
}

type mockEmailSender struct {
	err    error
	called bool
}

func (m *mockEmailSender) SendAlert(_ context.Context, _, _, _ string) error {
	m.called = true
	return m.err
}

type mockContactResolver struct {
	contacts []application.PatientContact
	err      error
}

func (m *mockContactResolver) GetContacts(_ context.Context, _ string) ([]application.PatientContact, error) {
	return m.contacts, m.err
}

func newUseCase(repo *mockAlertRepository, push *mockPushNotifier, email *mockEmailSender, resolver *mockContactResolver) *application.CreateAlertUseCase {
	return application.NewCreateAlertUseCase(repo, push, email, resolver, zap.NewNop())
}

func validEvent() sharedevents.AnomalyDetectedEvent {
	return sharedevents.AnomalyDetectedEvent{
		PatientID:   "patient-01",
		ReadingID:   "reading-001",
		MaxSeverity: sharedevents.SeverityWarning,
		HeartRate:   120,
		SpO2:        98.0,
		Violations: []sharedevents.RuleViolation{
			{RuleID: "r1", RuleName: "Taquicardia", MetricName: "heartRate", ActualValue: 120, Threshold: 100},
		},
	}
}

func contacts() []application.PatientContact {
	return []application.PatientContact{
		{PatientID: "patient-01", FCMToken: "fcm-token-123", Email: "patient@example.com"},
	}
}

// --- Tests ---

func TestCreateAlert_Success(t *testing.T) {
	repo := &mockAlertRepository{}
	push := &mockPushNotifier{}
	email := &mockEmailSender{}
	resolver := &mockContactResolver{contacts: contacts()}
	uc := newUseCase(repo, push, email, resolver)

	err := uc.Execute(context.Background(), validEvent())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.savedID == "" {
		t.Error("alert should be persisted")
	}
	if !push.called {
		t.Error("push notification should be sent")
	}
	if !email.called {
		t.Error("email notification should be sent")
	}
}

func TestCreateAlert_InvalidEvent_NoPatientID(t *testing.T) {
	event := validEvent()
	event.PatientID = ""
	uc := newUseCase(&mockAlertRepository{}, &mockPushNotifier{}, &mockEmailSender{}, &mockContactResolver{contacts: contacts()})

	err := uc.Execute(context.Background(), event)
	if err == nil {
		t.Fatal("expected error for event with empty patientID")
	}
}

func TestCreateAlert_RepositoryError(t *testing.T) {
	repo := &mockAlertRepository{saveErr: errors.New("cosmos down")}
	uc := newUseCase(repo, &mockPushNotifier{}, &mockEmailSender{}, &mockContactResolver{contacts: contacts()})

	err := uc.Execute(context.Background(), validEvent())
	if err == nil {
		t.Fatal("expected error when repository fails")
	}
}

func TestCreateAlert_ContactResolverError_DoesNotFail(t *testing.T) {
	resolver := &mockContactResolver{err: errors.New("user-service down")}
	push := &mockPushNotifier{}
	uc := newUseCase(&mockAlertRepository{}, push, &mockEmailSender{}, resolver)

	err := uc.Execute(context.Background(), validEvent())
	if err != nil {
		t.Fatalf("contact resolver error should not fail the operation, got: %v", err)
	}
	if push.called {
		t.Error("push should not be called when contacts cannot be resolved")
	}
}

func TestCreateAlert_NoContacts_DoesNotFail(t *testing.T) {
	resolver := &mockContactResolver{contacts: []application.PatientContact{}}
	push := &mockPushNotifier{}
	uc := newUseCase(&mockAlertRepository{}, push, &mockEmailSender{}, resolver)

	err := uc.Execute(context.Background(), validEvent())
	if err != nil {
		t.Fatalf("no contacts should not fail the operation, got: %v", err)
	}
	if push.called {
		t.Error("push should not be called when there are no contacts")
	}
}

func TestCreateAlert_PushFails_EmailStillSent(t *testing.T) {
	resolver := &mockContactResolver{contacts: contacts()}
	push := &mockPushNotifier{err: errors.New("FCM error")}
	email := &mockEmailSender{}
	uc := newUseCase(&mockAlertRepository{}, push, email, resolver)

	err := uc.Execute(context.Background(), validEvent())
	if err != nil {
		t.Fatalf("push failure should not abort the operation, got: %v", err)
	}
	if !email.called {
		t.Error("email should still be sent even if push fails")
	}
}

func TestCreateAlert_EmailFails_OperationSucceeds(t *testing.T) {
	resolver := &mockContactResolver{contacts: contacts()}
	email := &mockEmailSender{err: errors.New("ACS error")}
	uc := newUseCase(&mockAlertRepository{}, &mockPushNotifier{}, email, resolver)

	err := uc.Execute(context.Background(), validEvent())
	if err != nil {
		t.Fatalf("email failure should not abort the operation, got: %v", err)
	}
}

func TestCreateAlert_CriticalSeverity_Succeeds(t *testing.T) {
	event := validEvent()
	event.MaxSeverity = sharedevents.SeverityCritical
	resolver := &mockContactResolver{contacts: contacts()}
	push := &mockPushNotifier{}
	uc := newUseCase(&mockAlertRepository{}, push, &mockEmailSender{}, resolver)

	err := uc.Execute(context.Background(), event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !push.called {
		t.Error("push should be called for critical alert")
	}
}
