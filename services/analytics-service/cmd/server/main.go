package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos"
	"github.com/sentinel-health-engine/analytics-service/internal/cosmosdb"
	apphttp "github.com/sentinel-health-engine/analytics-service/internal/http"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	logger := buildLogger()
	defer logger.Sync() //nolint:errcheck

	logger = logger.With(zap.String("service", "analytics-service"))

	// ── Required environment variables ────────────────────────────────────────
	cosmosEndpoint := mustEnv(logger, "COSMOS_ENDPOINT")
	cosmosKey := mustEnv(logger, "COSMOS_KEY")
	jwtSecret := mustEnv(logger, "JWT_SECRET")

	// ── Optional with defaults ─────────────────────────────────────────────────
	cosmosDatabase := envOrDefault("COSMOS_DATABASE", "sentinel-health")
	telemetryContainer := envOrDefault("COSMOS_TELEMETRY_CONTAINER", "telemetry")
	alertsContainer := envOrDefault("COSMOS_ALERTS_CONTAINER", "alerts")
	port := envOrDefault("PORT", "8080")

	// jwtSecret is validated by mustEnv above; middleware reads it via os.Getenv at request time.
	_ = jwtSecret

	// ── Cosmos DB client ───────────────────────────────────────────────────────
	cred, err := azcosmos.NewKeyCredential(cosmosKey)
	if err != nil {
		logger.Fatal("failed to create Cosmos DB credential", zap.Error(err))
	}

	cosmosClient, err := azcosmos.NewClientWithKey(cosmosEndpoint, cred, nil)
	if err != nil {
		logger.Fatal("failed to create Cosmos DB client", zap.Error(err))
	}

	vitalsContainerClient, err := cosmosClient.NewContainer(cosmosDatabase, telemetryContainer)
	if err != nil {
		logger.Fatal("failed to get telemetry container client", zap.Error(err))
	}

	alertsContainerClient, err := cosmosClient.NewContainer(cosmosDatabase, alertsContainer)
	if err != nil {
		logger.Fatal("failed to get alerts container client", zap.Error(err))
	}

	vitalsRepo := cosmosdb.NewVitalsRepository(vitalsContainerClient)
	alertsRepo := cosmosdb.NewAlertsRepository(alertsContainerClient)

	// ── HTTP server ────────────────────────────────────────────────────────────
	router := apphttp.NewServer(vitalsRepo, alertsRepo, logger)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in background
	go func() {
		logger.Info("analytics-service starting", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	logger.Info("shutting down analytics-service...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}

	logger.Info("analytics-service stopped")
}

// mustEnv returns the value of an environment variable or fatally logs and exits.
func mustEnv(logger *zap.Logger, key string) string {
	v := os.Getenv(key)
	if v == "" {
		logger.Fatal("required environment variable not set", zap.String("key", key))
	}
	return v
}

// envOrDefault returns the value of an environment variable or a default value.
func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// buildLogger creates a zap logger using the LOG_LEVEL environment variable.
func buildLogger() *zap.Logger {
	level := zapcore.InfoLevel
	if l := os.Getenv("LOG_LEVEL"); l != "" {
		if err := level.UnmarshalText([]byte(l)); err != nil {
			// Fall back to info
			level = zapcore.InfoLevel
		}
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(level)
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := cfg.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to build logger: %v", err))
	}
	return logger
}
