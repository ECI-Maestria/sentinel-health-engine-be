// Package http configures and starts the Gin HTTP server.
package http

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sentinel-health-engine/user-service/internal/infrastructure/http/docs"
	"github.com/sentinel-health-engine/user-service/internal/infrastructure/http/handlers"
)

// Server wraps the Gin engine and all registered handlers.
type Server struct {
	engine *gin.Engine
}

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
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Internal-API-Key")
			c.Header("Access-Control-Max-Age", maxAge)
			c.Header("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// NewServer builds the Gin engine with all routes registered.
func NewServer(
	authHandler      *handlers.AuthHandler,
	userHandler      *handlers.UserHandler,
	patientHandler   *handlers.PatientHandler,
	deviceHandler    *handlers.DeviceHandler,
	caretakerHandler *handlers.CaretakerHandler,
	doctorHandler    *handlers.DoctorHandler,
	internalHandler  *handlers.InternalHandler,
	dashboardHandler *handlers.DashboardHandler,
) *Server {
	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(corsMiddleware())

	v1 := engine.Group("/v1")

	authHandler.Register(v1)
	userHandler.Register(v1)
	patientHandler.Register(v1)
	deviceHandler.Register(v1)
	caretakerHandler.Register(v1)
	doctorHandler.Register(v1)
	internalHandler.Register(v1)
	dashboardHandler.Register(v1)

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "user-service"})
	})

	// ── API documentation (no auth required) ─────────────────────────────
	engine.GET("/docs", docs.ServeUI)
	engine.GET("/openapi.yaml", docs.ServeSpec)

	return &Server{engine: engine}
}

// Handler returns the underlying http.Handler for use with net/http.
func (s *Server) Handler() http.Handler {
	return s.engine
}
