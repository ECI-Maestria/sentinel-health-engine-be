package http

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sentinel-health-engine/analytics-service/internal/cosmosdb"
	"github.com/sentinel-health-engine/analytics-service/internal/http/docs"
	"github.com/sentinel-health-engine/analytics-service/internal/http/handlers"
	"github.com/sentinel-health-engine/analytics-service/internal/http/middleware"
	"go.uber.org/zap"
)

// corsMiddleware handles Cross-Origin Resource Sharing.
// Allowed origins are read from the ALLOWED_ORIGINS env var (comma-separated).
// Use "*" or leave empty to allow all origins (development only).
func corsMiddleware() gin.HandlerFunc {
	rawOrigins := os.Getenv("ALLOWED_ORIGINS")
	allowAll := rawOrigins == "" || rawOrigins == "*"
	var allowedOrigins []string
	if !allowAll {
		for _, o := range strings.Split(rawOrigins, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}
	maxAge := strconv.Itoa(int(12 * time.Hour / time.Second))

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			c.Next()
			return
		}
		permitted := allowAll
		if !permitted {
			for _, o := range allowedOrigins {
				if o == origin {
					permitted = true
					break
				}
			}
		}
		if permitted {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
			c.Header("Access-Control-Max-Age", maxAge)
			c.Header("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			if permitted {
				c.AbortWithStatus(http.StatusNoContent)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}
		c.Next()
	}
}

// NewServer creates and configures a Gin engine with all routes registered.
func NewServer(
	vitalsRepo *cosmosdb.VitalsRepository,
	alertsRepo *cosmosdb.AlertsRepository,
	logger *zap.Logger,
) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())
	router.Use(ginZapLogger(logger))

	// Health check (no auth required)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	vitalsHandler := handlers.NewVitalsHandler(vitalsRepo, logger)
	alertsHandler := handlers.NewAlertsHandler(alertsRepo, logger)
	reportsHandler := handlers.NewReportsHandler(vitalsRepo, alertsRepo, logger)

	v1 := router.Group("/v1")

	// Vitals routes — any authenticated user
	patients := v1.Group("/patients/:id")
	patients.Use(middleware.RequireAuth())
	{
		patients.GET("/vitals/history", vitalsHandler.GetHistory)
		patients.GET("/vitals/latest", vitalsHandler.GetLatest)
		patients.GET("/vitals/summary", vitalsHandler.GetSummary)

		patients.GET("/alerts/history", alertsHandler.GetHistory)
		patients.GET("/alerts/stats", alertsHandler.GetStats)
		patients.PATCH("/alerts/:alertId/acknowledge", alertsHandler.Acknowledge)

		// Reports — DOCTOR or CARETAKER only
		patients.POST(
			"/reports/generate",
			middleware.RequireRole("DOCTOR", "CARETAKER"),
			reportsHandler.Generate,
		)
	}

	router.GET("/docs", docs.ServeUI)
	router.GET("/openapi.yaml", docs.ServeSpec)

	return router
}

// ginZapLogger returns a Gin middleware that logs requests using zap.
func ginZapLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		logger.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.String("clientIP", c.ClientIP()),
		)
	}
}
