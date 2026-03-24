package middleware

import (
	"net/http"
	"strings"

	"project-manager/backend/internal/auth"
	"project-manager/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

func loadUserPermissions(db *gorm.DB, userID uint) ([]string, error) {
	if db == nil {
		return nil, nil
	}
	var user model.User
	if err := db.Preload("Roles.Permissions").Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	permissionSet := map[string]struct{}{}
	for _, role := range user.Roles {
		for _, permission := range role.Permissions {
			permissionSet[permission.Code] = struct{}{}
		}
	}
	permissions := make([]string, 0, len(permissionSet))
	for code := range permissionSet {
		permissions = append(permissions, code)
	}
	return permissions, nil
}

func RequirePermission(db *gorm.DB, permission string) gin.HandlerFunc {
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

		effectivePermissions := claims.Permissions
		if db != nil {
			permissions, err := loadUserPermissions(db, claims.UserID)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "user not found"})
				return
			}
			if permissions != nil {
				effectivePermissions = permissions
			}
		}

		if hasPermission(effectivePermissions, permission) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "forbidden"})
	}
}
