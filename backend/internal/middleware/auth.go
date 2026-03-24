package middleware

import (
	"net/http"
	"strings"

	"project-manager/backend/internal/auth"

	"github.com/gin-gonic/gin"
)

func hasPermission(granted []string, required string) bool {
	for _, item := range granted {
		if item == required {
			return true
		}
	}
	if strings.HasSuffix(required, ".read") {
		writePerm := strings.TrimSuffix(required, ".read") + ".write"
		for _, item := range granted {
			if item == writePerm {
				return true
			}
		}
	}
	return false
}

func JWT(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "missing or invalid authorization header"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := auth.ParseToken(secret, tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "invalid token"})
			return
		}

		c.Set("claims", claims)
		c.Set("userId", claims.UserID)
		c.Next()
	}
}

func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get("claims")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
			return
		}

		claims, ok := value.(*auth.Claims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "unauthorized"})
			return
		}

		if hasPermission(claims.Permissions, permission) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "forbidden"})
	}
}
