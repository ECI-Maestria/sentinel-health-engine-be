// main.go — Composition root for the Telemetry Service.
// Wires all dependencies (Clean Architecture: DI at the outermost boundary).
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
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/sentinel-health-engine/telemetry-service/internal/application"
	"github.com/sentinel-health-engine/telemetry-service/internal/infrastructure/cosmosdb"
	"github.com/sentinel-health-engine/telemetry-service/internal/infrastructure/deviceregistry"
	"github.com/sentinel-health-engine/telemetry-service/internal/infrastructure/iothub"
	sbpublisher "github.com/sentinel-health-engine/telemetry-service/internal/infrastructure/servicebus"
	"github.com/sentinel-health-engine/shared/pkg/logger"
)

func main() {
	log := logger.MustNew("telemetry-service")
	defer log.Sync() //nolint:errcheck

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, log); err != nil {
		log.Fatal("service terminated with error", zap.Error(err))
	}
	log.Info("service shut down gracefully")
}

func run(ctx context.Context, log *zap.Logger) error {
	// ── Service Bus ─────────────────────────────────────────────────────────
	sbClient, err := azservicebus.NewClientFromConnectionString(mustEnv("SERVICE_BUS_CONNECTION_STRING"), nil)
	if err != nil {
		return fmt.Errorf("service bus client: %w", err)
	}
	publisher := sbpublisher.NewTelemetryEventPublisher(sbClient, mustEnv("TELEMETRY_TOPIC_NAME"), log)

	// ── Cosmos DB ────────────────────────────────────────────────────────────
	cosmosCred, err := azcosmos.NewKeyCredential(mustEnv("COSMOS_KEY"))
	if err != nil {
		return fmt.Errorf("cosmos credential: %w", err)
	}
	cosmosClient, err := azcosmos.NewClientWithKey(
		mustEnv("COSMOS_ENDPOINT"),
		cosmosCred,
		nil,
	)
	if err != nil {
		return fmt.Errorf("cosmos client: %w", err)
	}
	containerClient, err := cosmosClient.NewContainer(mustEnv("COSMOS_DATABASE"), mustEnv("COSMOS_CONTAINER"))
	if err != nil {
		return fmt.Errorf("cosmos container client: %w", err)
	}
	repo := cosmosdb.NewTelemetryCosmosRepository(containerClient)

	// ── Device Registry ──────────────────────────────────────────────────────
	registry, err := deviceregistry.NewUserServiceRegistry()
	if err != nil {
		return fmt.Errorf("device registry: %w", err)
	}

	// ── Use Case ─────────────────────────────────────────────────────────────
	ingestUseCase := application.NewIngestTelemetryUseCase(repo, registry, publisher, log)

	// ── IoT Hub Consumer ─────────────────────────────────────────────────────
	consumer, err := iothub.NewConsumer(
		mustEnv("IOTHUB_EVENTHUB_CONNECTION_STRING"),
		mustEnv("IOTHUB_EVENTHUB_NAME"),
		mustEnv("IOTHUB_CONSUMER_GROUP"),
		ingestUseCase,
		log,
	)
	if err != nil {
		return fmt.Errorf("IoT Hub consumer: %w", err)
	}

	// ── HTTP Health Check ────────────────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "telemetry-service"})
	})

	port := getEnv("PORT", "8080")
	srv := &http.Server{Addr: ":" + port, Handler: router}

	go func() {
		log.Info("HTTP listening", zap.String("port", port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP error", zap.Error(err))
		}
	}()

	// ── Start IoT Hub consumer ───────────────────────────────────────────────
	go func() {
		if err := consumer.Start(ctx); err != nil && ctx.Err() == nil {
			log.Error("IoT Hub consumer stopped unexpectedly", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutdown signal received")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
