package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// RequireInternalAPIKey validates the X-Internal-API-Key header.
// Used for service-to-service communication (telemetry-service, alerts-service).
func RequireInternalAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		expectedKey := os.Getenv("INTERNAL_API_KEY")
		if expectedKey == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal API key not configured"})
			return
		}

		provided := c.GetHeader("X-Internal-API-Key")
		if provided == "" || provided != expectedKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or missing internal API key"})
			return
		}

		c.Next()
	}
}
