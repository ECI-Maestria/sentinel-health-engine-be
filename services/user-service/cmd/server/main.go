// Package main is the entry point for the user-service.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	authapp "github.com/sentinel-health-engine/user-service/internal/application/auth"
	carapp "github.com/sentinel-health-engine/user-service/internal/application/caretaker"
	devapp "github.com/sentinel-health-engine/user-service/internal/application/device"
	docapp "github.com/sentinel-health-engine/user-service/internal/application/doctor"
	intapp "github.com/sentinel-health-engine/user-service/internal/application/svcinternal"
	patapp "github.com/sentinel-health-engine/user-service/internal/application/patient"
	usrapp "github.com/sentinel-health-engine/user-service/internal/application/user"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/email"
	httpinfra "github.com/sentinel-health-engine/user-service/internal/infrastructure/http"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/http/handlers"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/iothub"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/postgres"
	"github.com/sentinel-health-engine/shared/pkg/logger"
)

func main() {
	log := logger.MustNew("user-service")
	defer log.Sync() //nolint:errcheck

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── Postgres ──────────────────────────────────────────────────────────────
	pool, err := postgres.Connect(ctx, log)
	if err != nil {
		log.Fatal("failed to connect to postgres", zap.Error(err))
	}
	defer pool.Close()

	// ── Repositories ──────────────────────────────────────────────────────────
	userRepo           := postgres.NewUserRepository(pool)
	deviceRepo         := postgres.NewDeviceRepository(pool)
	caretakerRepo      := postgres.NewCaretakerRepository(pool)
	passwordResetRepo  := postgres.NewPasswordResetRepository(pool)

	// ── Email sender (optional — graceful degradation) ─────────────────────────
	var (
		welcomeSender patapp.WelcomeEmailSender
		resetSender   authapp.PasswordResetEmailSender
	)
	acsSender, err := email.NewACSSender(log)
	if err != nil {
		log.Warn("ACS email sender unavailable — emails will be skipped", zap.Error(err))
		noop := &noopEmailSender{}
		welcomeSender = noop
		resetSender = noop
	} else {
		welcomeSender = acsSender
		resetSender = acsSender
	}

	// ── IoT Hub registry (optional — graceful degradation if not configured) ──
	var iotRegistry devapp.IoTHubRegistry
	if connStr := os.Getenv("IOTHUB_CONNECTION_STRING"); connStr != "" {
		reg, err := iothub.NewRegistryFromConnectionString(connStr)
		if err != nil {
			log.Warn("IoT Hub registry unavailable — device IoT registration will be skipped", zap.Error(err))
		} else {
			iotRegistry = reg
			log.Info("IoT Hub registry configured")
		}
	} else {
		log.Warn("IOTHUB_CONNECTION_STRING not set — device IoT registration will be skipped")
	}

	// ── Use cases ─────────────────────────────────────────────────────────────
	loginUC          := authapp.NewLoginUseCase(userRepo, log)
	refreshUC        := authapp.NewRefreshUseCase(userRepo, log)
	changePasswordUC := authapp.NewChangePasswordUseCase(userRepo, log)
	forgotPasswordUC := authapp.NewRequestPasswordResetUseCase(userRepo, passwordResetRepo, resetSender, log)
	verifyCodeUC     := authapp.NewVerifyResetCodeUseCase(userRepo, passwordResetRepo, log)
	resetPasswordUC  := authapp.NewResetPasswordUseCase(userRepo, passwordResetRepo, log)

	createPatientUC  := patapp.NewCreatePatientUseCase(userRepo, welcomeSender, log)
	listPatientsUC   := patapp.NewListPatientsUseCase(userRepo)
	getPatientUC     := patapp.NewGetPatientUseCase(userRepo)

	registerDeviceUC := devapp.NewRegisterDeviceUseCase(deviceRepo, iotRegistry, log)
	listDevicesUC    := devapp.NewListDevicesUseCase(deviceRepo)

	createCaretakerUC  := carapp.NewCreateCaretakerUseCase(userRepo, welcomeSender, log)
	linkCaretakerUC    := carapp.NewLinkCaretakerUseCase(caretakerRepo, userRepo, log)
	unlinkCaretakerUC  := carapp.NewUnlinkCaretakerUseCase(caretakerRepo, log)
	listCaretakersUC   := carapp.NewListCaretakersUseCase(caretakerRepo, userRepo)
	getMyPatientsUC    := carapp.NewGetMyPatientsUseCase(caretakerRepo, userRepo, log)

	createDoctorUC   := docapp.NewCreateDoctorUseCase(userRepo, welcomeSender, log)
	listDoctorsUC    := docapp.NewListDoctorsUseCase(userRepo)

	getMeUC          := usrapp.NewGetMeUseCase(userRepo)
	getDashboardUC   := usrapp.NewGetDoctorDashboardUseCase(userRepo, listDevicesUC, listCaretakersUC)

	validateDeviceUC  := intapp.NewValidateDeviceUseCase(deviceRepo, userRepo)
	getContactsUC     := intapp.NewGetPatientContactsUseCase(userRepo, deviceRepo, caretakerRepo)

	// ── HTTP handlers ─────────────────────────────────────────────────────────
	authHandler      := handlers.NewAuthHandler(loginUC, refreshUC, changePasswordUC, forgotPasswordUC, verifyCodeUC, resetPasswordUC)
	userHandler      := handlers.NewUserHandler(getMeUC)
	patientHandler   := handlers.NewPatientHandler(createPatientUC, listPatientsUC, getPatientUC, listDevicesUC, listCaretakersUC)
	deviceHandler    := handlers.NewDeviceHandler(registerDeviceUC, listDevicesUC)
	caretakerHandler := handlers.NewCaretakerHandler(createCaretakerUC, linkCaretakerUC, unlinkCaretakerUC, listCaretakersUC, getMyPatientsUC)
	doctorHandler    := handlers.NewDoctorHandler(createDoctorUC, listDoctorsUC)
	internalHandler   := handlers.NewInternalHandler(validateDeviceUC, getContactsUC)
	dashboardHandler  := handlers.NewDashboardHandler(getDashboardUC)

	// ── HTTP server ───────────────────────────────────────────────────────────
	srv := httpinfra.NewServer(authHandler, userHandler, patientHandler, deviceHandler, caretakerHandler, doctorHandler, internalHandler, dashboardHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: srv.Handler(),
	}

	go func() {
		log.Info("user-service listening", zap.String("port", port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("http server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down user-service")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("http server shutdown error", zap.Error(err))
	}
}

// noopEmailSender discards all emails when ACS is not configured.
type noopEmailSender struct{}

func (n *noopEmailSender) SendWelcome(_ context.Context, _, _, _ string) error       { return nil }
func (n *noopEmailSender) SendPasswordResetCode(_ context.Context, _, _, _ string) error { return nil }
