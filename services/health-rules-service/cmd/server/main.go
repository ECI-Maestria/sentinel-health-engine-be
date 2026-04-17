package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/sentinel-health-engine/health-rules-service/internal/application"
	"github.com/sentinel-health-engine/health-rules-service/internal/infrastructure/servicebus"
	"github.com/sentinel-health-engine/shared/pkg/logger"
)

func main() {
	log := logger.MustNew("health-rules-service")
	defer log.Sync() //nolint:errcheck

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, log, cancel); err != nil {
		log.Fatal("service error", zap.Error(err))
	}
}

func run(ctx context.Context, log *zap.Logger, cancel context.CancelFunc) error {
	sbClient, err := azservicebus.NewClientFromConnectionString(mustEnv("SERVICE_BUS_CONNECTION_STRING"), nil)
	if err != nil {
		return fmt.Errorf("service bus: %w", err)
	}

	publisher := servicebus.NewAnomalyPublisher(sbClient, mustEnv("ANOMALY_TOPIC_NAME"), log)
	useCase := application.NewEvaluateRulesUseCase(publisher, log)
	consumer := servicebus.NewTelemetryConsumer(
		sbClient,
		mustEnv("TELEMETRY_TOPIC_NAME"),
		mustEnv("TELEMETRY_SUBSCRIPTION_NAME"),
		useCase, log,
	)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "health-rules-service"})
	})

	port := getEnv("PORT", "8080")
	srv := &http.Server{Addr: ":" + port, Handler: router}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP error", zap.Error(err))
		}
	}()

	go func() {
		if err := consumer.Start(ctx); err != nil && ctx.Err() == nil {
			log.Error("consumer error", zap.Error(err))
			cancel()
		}
	}()

	<-ctx.Done()
	shutdownCtx, sc := context.WithTimeout(context.Background(), 10*time.Second)
	defer sc()
	return srv.Shutdown(shutdownCtx)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required env var %q not set", key))
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
