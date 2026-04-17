package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	ctxKeyUserID = "userId"
	ctxKeyEmail  = "email"
	ctxKeyRole   = "role"
)

type Claims struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Type   string `json:"type"`
	jwt.RegisteredClaims
}

// RequireAuth validates the Bearer JWT and injects userId, email, role into the Gin context.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid Authorization header format"})
			return
		}

		tokenString := parts[1]
		secret := os.Getenv("JWT_SECRET")

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		if claims.Type != "access" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token type must be access"})
			return
		}

		c.Set(ctxKeyUserID, claims.UserID)
		c.Set(ctxKeyEmail, claims.Email)
		c.Set(ctxKeyRole, claims.Role)

		c.Next()
	}
}

// RequireRole returns middleware that allows only the specified roles.
// RequireAuth must be called before this middleware in the chain.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		role, exists := c.Get(ctxKeyRole)
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "role not found in context"})
			return
		}

		roleStr, ok := role.(string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid role type in context"})
			return
		}

		if _, permitted := allowed[roleStr]; !permitted {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}

		c.Next()
	}
}

// CurrentUserID retrieves the authenticated user ID from the Gin context.
func CurrentUserID(c *gin.Context) string {
	v, _ := c.Get(ctxKeyUserID)
	s, _ := v.(string)
	return s
}

// CurrentRole retrieves the authenticated user role from the Gin context.
func CurrentRole(c *gin.Context) string {
	v, _ := c.Get(ctxKeyRole)
	s, _ := v.(string)
	return s
}
