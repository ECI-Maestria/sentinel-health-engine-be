// Package main is the entry point for the calendar-service.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	calhttp "github.com/sentinel-health-engine/calendar-service/internal/http"
	"github.com/sentinel-health-engine/calendar-service/internal/http/handlers"
	calEmail "github.com/sentinel-health-engine/calendar-service/internal/infrastructure/email"
	calFirebase "github.com/sentinel-health-engine/calendar-service/internal/infrastructure/firebase"
	"github.com/sentinel-health-engine/calendar-service/internal/notifications"
	"github.com/sentinel-health-engine/calendar-service/internal/postgres"
)

func main() {
	logger := buildLogger()
	defer logger.Sync() //nolint:errcheck

	logger = logger.With(zap.String("service", "calendar-service"))

	// ── Required environment variables ─────────────────────────────────────────
	if os.Getenv("CALENDAR_DATABASE_URL") == "" {
		logger.Fatal("CALENDAR_DATABASE_URL is required")
	}
	if os.Getenv("JWT_SECRET") == "" {
		logger.Fatal("JWT_SECRET is required")
	}
	if os.Getenv("USER_SERVICE_URL") == "" {
		logger.Fatal("USER_SERVICE_URL is required")
	}
	if os.Getenv("USER_SERVICE_API_KEY") == "" {
		logger.Fatal("USER_SERVICE_API_KEY is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// ── Database ────────────────────────────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := postgres.Connect(ctx, logger)
	if err != nil {
		logger.Fatal("connect to postgres", zap.Error(err))
	}
	defer pool.Close()

	// ── Repositories ────────────────────────────────────────────────────────────
	apptRepo := postgres.NewAppointmentRepository(pool)
	medRepo := postgres.NewMedicationRepository(pool)
	remRepo := postgres.NewReminderRepository(pool)

	// ── Email sender (optional — warn and continue if ACS not configured) ───────
	var emailSender notifications.EmailSender
	emailSnd, err := calEmail.NewSender(logger)
	if err != nil {
		logger.Warn("email notifications disabled", zap.Error(err))
	} else {
		emailSender = emailSnd
	}

	// ── Firebase push notifier (optional — warn and continue if not configured) ─
	var pushNotifier notifications.PushNotifier
	pushNtf, err := calFirebase.NewPushNotifier(ctx, logger)
	if err != nil {
		logger.Warn("push notifications disabled", zap.Error(err))
	} else {
		pushNotifier = pushNtf
	}

	// ── Notification worker ─────────────────────────────────────────────────────
	worker := notifications.NewWorker(logger, apptRepo, remRepo, emailSender, pushNotifier)
	go worker.Start(ctx)

	// ── HTTP handlers ───────────────────────────────────────────────────────────
	apptHandler := handlers.NewAppointmentHandler(apptRepo, logger)
	medHandler := handlers.NewMedicationHandler(medRepo, logger)
	remHandler := handlers.NewReminderHandler(remRepo, logger)

	// ── Router ──────────────────────────────────────────────────────────────────
	router := calhttp.NewRouter(apptHandler, medHandler, remHandler)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Graceful shutdown ────────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		logger.Info("calendar-service listening", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("listen and serve", zap.Error(err))
		}
	}()

	<-quit
	logger.Info("shutting down calendar-service")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	cancel() // stop the notification worker

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	}

	logger.Info("calendar-service stopped")
}

// buildLogger creates a zap logger respecting LOG_LEVEL env var.
func buildLogger() *zap.Logger {
	level := zapcore.InfoLevel
	if lvl := os.Getenv("LOG_LEVEL"); lvl != "" {
		if err := level.UnmarshalText([]byte(lvl)); err != nil {
			level = zapcore.InfoLevel
		}
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(level)
	logger, err := cfg.Build()
	if err != nil {
		panic(fmt.Sprintf("build zap logger: %v", err))
	}
	return logger
}
