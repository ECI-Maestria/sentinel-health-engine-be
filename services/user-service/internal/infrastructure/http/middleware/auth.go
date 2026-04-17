// Package middleware contains Gin middleware for the user-service HTTP layer.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/sentinel-health-engine/user-service/internal/application/auth"
)

const (
	ContextKeyUserID = "userId"
	ContextKeyEmail  = "email"
	ContextKeyRole   = "role"
)

// RequireAuth validates the Bearer JWT and injects claims into the Gin context.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")
		claims, err := auth.ParseAccessToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyEmail, claims.Email)
		c.Set(ContextKeyRole, claims.Role)
		c.Next()
	}
}

// RequireRole aborts if the authenticated user does not have one of the allowed roles.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		role, _ := c.Get(ContextKeyRole)
		if !allowed[role.(string)] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}

// CurrentUserID extracts the user ID from the Gin context (set by RequireAuth).
func CurrentUserID(c *gin.Context) string {
	v, _ := c.Get(ContextKeyUserID)
	s, _ := v.(string)
	return s
}

// CurrentRole extracts the role from the Gin context.
func CurrentRole(c *gin.Context) string {
	v, _ := c.Get(ContextKeyRole)
	s, _ := v.(string)
	return s
}
