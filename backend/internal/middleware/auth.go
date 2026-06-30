package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"project-manager/backend/internal/auth"
	"project-manager/backend/internal/model"
	"project-manager/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	contextPermissionsLoadedKey = "permissionsLoaded"
	contextPermissionsKey       = "effectivePermissions"
	contextAuthTypeKey          = "authType"
	contextAPITokenIDKey        = "apiTokenId"
)

func hasPermission(granted []string, required string) bool {
	for _, item := range granted {
		if item == required {
			return true
		}
	}
	return false
}

func JWT(secret string, dbs ...*gorm.DB) gin.HandlerFunc {
	var db *gorm.DB
	if len(dbs) > 0 {
		db = dbs[0]
	}
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "missing or invalid authorization header"})
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		claims, err := auth.ParseToken(secret, tokenString)
		if err == nil {
			c.Set("claims", claims)
			c.Set("userId", claims.UserID)
			c.Set(contextAuthTypeKey, "jwt")
			c.Next()
			return
		}

		if db != nil && util.IsAPIToken(tokenString) {
			apiClaims, apiToken, apiPermissions, apiErr := authenticateAPIToken(db, tokenString)
			if apiErr == nil {
				now := time.Now()
				_ = db.Model(&model.APIToken{}).
					Where("id = ?", apiToken.ID).
					Updates(map[string]any{"last_used_at": now, "last_used_ip": c.ClientIP()}).Error
				c.Set("claims", apiClaims)
				c.Set("userId", apiClaims.UserID)
				c.Set(contextAuthTypeKey, "api_token")
				c.Set(contextAPITokenIDKey, apiToken.ID)
				c.Set(contextPermissionsLoadedKey, true)
				c.Set(contextPermissionsKey, apiPermissions)
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "invalid token"})
	}
}

func authenticateAPIToken(db *gorm.DB, tokenString string) (*auth.Claims, model.APIToken, []string, error) {
	prefix := util.APITokenLookupPrefix(tokenString)
	var tokens []model.APIToken
	if err := db.Preload("ServiceAccount.Roles.Permissions").
		Where("token_prefix = ?", prefix).
		Find(&tokens).Error; err != nil {
		return nil, model.APIToken{}, nil, err
	}
	now := time.Now()
	for _, token := range tokens {
		if !util.EqualAPITokenHash(token.TokenHash, tokenString) {
			continue
		}
		if !token.IsEnabled || token.RevokedAt != nil {
			return nil, model.APIToken{}, nil, errors.New("api token disabled")
		}
		if token.ExpiresAt != nil && token.ExpiresAt.Before(now) {
			return nil, model.APIToken{}, nil, errors.New("api token expired")
		}
		if !token.ServiceAccount.IsActive {
			return nil, model.APIToken{}, nil, errors.New("service account disabled")
		}
		rolePermissions := userPermissionCodes(token.ServiceAccount)
		effectivePermissions := intersectPermissions(rolePermissions, token.PermissionCodes)
		claims := &auth.Claims{
			UserID:      token.ServiceAccountID,
			Username:    token.ServiceAccount.Username,
			Permissions: effectivePermissions,
		}
		return claims, token, effectivePermissions, nil
	}
	return nil, model.APIToken{}, nil, errors.New("api token not found")
}

func userPermissionCodes(user model.User) []string {
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
	return permissions
}

func intersectPermissions(granted []string, allowed []string) []string {
	if len(granted) == 0 || len(allowed) == 0 {
		return nil
	}
	grantedSet := map[string]struct{}{}
	for _, permission := range granted {
		grantedSet[permission] = struct{}{}
	}
	effective := make([]string, 0, len(allowed))
	seen := map[string]struct{}{}
	for _, permission := range allowed {
		if _, ok := grantedSet[permission]; !ok {
			continue
		}
		if _, exists := seen[permission]; exists {
			continue
		}
		seen[permission] = struct{}{}
		effective = append(effective, permission)
	}
	return effective
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

		effectivePermissions, err := resolveEffectivePermissions(c, db, claims)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED", "message": "user not found"})
			return
		}

		if hasPermission(effectivePermissions, permission) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": "FORBIDDEN", "message": "forbidden"})
	}
}

func resolveEffectivePermissions(c *gin.Context, db *gorm.DB, claims *auth.Claims) ([]string, error) {
	if loaded, ok := c.Get(contextPermissionsLoadedKey); ok {
		if value, yes := loaded.(bool); yes && value {
			if cached, exists := c.Get(contextPermissionsKey); exists {
				if permissions, castOK := cached.([]string); castOK {
					return permissions, nil
				}
			}
		}
	}

	effectivePermissions := claims.Permissions
	if db != nil {
		permissions, err := loadUserPermissions(db, claims.UserID)
		if err != nil {
			return nil, err
		}
		if permissions != nil {
			effectivePermissions = permissions
		}
	}

	c.Set(contextPermissionsLoadedKey, true)
	c.Set(contextPermissionsKey, effectivePermissions)
	return effectivePermissions, nil
}
