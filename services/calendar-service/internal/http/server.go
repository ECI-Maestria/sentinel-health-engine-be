// Package http wires the Gin router for the calendar-service.
package http

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/sentinel-health-engine/calendar-service/internal/http/docs"
	"github.com/sentinel-health-engine/calendar-service/internal/http/handlers"
	"github.com/sentinel-health-engine/calendar-service/internal/http/middleware"
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

// NewRouter creates and configures the Gin engine with all calendar-service routes.
func NewRouter(
	apptHandler *handlers.AppointmentHandler,
	medHandler *handlers.MedicationHandler,
	remHandler *handlers.ReminderHandler,
) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(corsMiddleware())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "calendar-service"})
	})

	v1 := router.Group("/v1")

	// ── Appointments ────────────────────────────────────────────────────────────
	appointments := v1.Group("/patients/:id/appointments")
	{
		appointments.POST("",
			middleware.RequireAuth(),
			middleware.RequireRole("DOCTOR"),
			apptHandler.Create,
		)
		appointments.GET("",
			middleware.RequireAuth(),
			apptHandler.List,
		)
		appointments.GET("/:apptId",
			middleware.RequireAuth(),
			apptHandler.GetByID,
		)
		appointments.PUT("/:apptId",
			middleware.RequireAuth(),
			middleware.RequireRole("DOCTOR"),
			apptHandler.Update,
		)
		appointments.DELETE("/:apptId",
			middleware.RequireAuth(),
			middleware.RequireRole("DOCTOR"),
			apptHandler.Delete,
		)
	}

	// ── Medications ─────────────────────────────────────────────────────────────
	medications := v1.Group("/patients/:id/medications")
	{
		medications.POST("",
			middleware.RequireAuth(),
			middleware.RequireRole("DOCTOR"),
			medHandler.Create,
		)
		medications.GET("",
			middleware.RequireAuth(),
			medHandler.List,
		)
		medications.GET("/:medId",
			middleware.RequireAuth(),
			medHandler.GetByID,
		)
		medications.PUT("/:medId",
			middleware.RequireAuth(),
			middleware.RequireRole("DOCTOR"),
			medHandler.Update,
		)
		medications.DELETE("/:medId",
			middleware.RequireAuth(),
			middleware.RequireRole("DOCTOR"),
			medHandler.Deactivate,
		)
	}

	// ── Reminders ───────────────────────────────────────────────────────────────
	reminders := v1.Group("/patients/:id/reminders")
	{
		reminders.POST("",
			middleware.RequireAuth(),
			remHandler.Create,
		)
		reminders.GET("",
			middleware.RequireAuth(),
			remHandler.List,
		)
		reminders.GET("/today",
			middleware.RequireAuth(),
			remHandler.ListToday,
		)
		reminders.GET("/:remId",
			middleware.RequireAuth(),
			remHandler.GetByID,
		)
		reminders.PUT("/:remId",
			middleware.RequireAuth(),
			remHandler.Update,
		)
		reminders.DELETE("/:remId",
			middleware.RequireAuth(),
			remHandler.Cancel,
		)
	}

	router.GET("/docs", docs.ServeUI)
	router.GET("/openapi.yaml", docs.ServeSpec)

	return router
}
