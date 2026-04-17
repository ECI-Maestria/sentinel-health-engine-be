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

	"github.com/sentinel-health-engine/alerts-service/internal/application"
	cosmosrepo "github.com/sentinel-health-engine/alerts-service/internal/infrastructure/cosmosdb"
	"github.com/sentinel-health-engine/alerts-service/internal/infrastructure/email"
	"github.com/sentinel-health-engine/alerts-service/internal/infrastructure/firebase"
	sbconsumer "github.com/sentinel-health-engine/alerts-service/internal/infrastructure/servicebus"
	"github.com/sentinel-health-engine/alerts-service/internal/infrastructure/userservice"
	"github.com/sentinel-health-engine/shared/pkg/logger"
)

func main() {
	log := logger.MustNew("alerts-service")
	defer log.Sync() //nolint:errcheck

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx, log, cancel); err != nil {
		log.Fatal("service error", zap.Error(err))
	}
}

func run(ctx context.Context, log *zap.Logger, cancel context.CancelFunc) error {
	// ── Service Bus ──────────────────────────────────────────────────────────
	sbClient, err := azservicebus.NewClientFromConnectionString(mustEnv("SERVICE_BUS_CONNECTION_STRING"), nil)
	if err != nil {
		return fmt.Errorf("service bus: %w", err)
	}

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
	alertsContainer, err := cosmosClient.NewContainer(mustEnv("COSMOS_DATABASE"), mustEnv("COSMOS_ALERTS_CONTAINER"))
	if err != nil {
		return fmt.Errorf("cosmos container: %w", err)
	}
	repo := cosmosrepo.NewAlertCosmosRepository(alertsContainer)

	// ── Firebase (optional — graceful degradation) ────────────────────────────
	var pushNotifier application.PushNotifier
	fcm, err := firebase.NewPushNotifier(ctx, log)
	if err != nil {
		log.Warn("Firebase not configured — push notifications disabled", zap.Error(err))
	} else {
		pushNotifier = fcm
	}

	// ── ACS Email (optional — graceful degradation) ───────────────────────────
	var emailSender application.EmailSender
	acs, err := email.NewACSEmailSender(log)
	if err != nil {
		log.Warn("ACS email not configured — email notifications disabled", zap.Error(err))
	} else {
		emailSender = acs
	}

	// ── Contact Resolver (user-service) ──────────────────────────────────────
	resolver, err := userservice.NewContactResolver()
	if err != nil {
		return fmt.Errorf("contact resolver: %w", err)
	}
	contactResolver := userservice.NewContactResolverAdapter(resolver)

	// ── Use Case ─────────────────────────────────────────────────────────────
	useCase := application.NewCreateAlertUseCase(repo, pushNotifier, emailSender, contactResolver, log)

	// ── Consumer ─────────────────────────────────────────────────────────────
	consumer := sbconsumer.NewAnomalyConsumer(
		sbClient,
		mustEnv("ANOMALY_TOPIC_NAME"),
		mustEnv("ANOMALY_SUBSCRIPTION_NAME"),
		useCase, log,
	)

	// ── HTTP Health Check ────────────────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "alerts-service"})
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
