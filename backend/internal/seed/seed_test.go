package seed

import (
	"bytes"
	"log"
	"reflect"
	"sort"
	"strings"
	"testing"

	"project-manager/backend/internal/database"
	"project-manager/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return openSeedTestDBWithConfig(t, &gorm.Config{})
}

func openSeedTestDBWithConfig(t *testing.T, cfg *gorm.Config) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), cfg)
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return db
}

func permissionCodes(permissions []model.Permission) []string {
	codes := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		codes = append(codes, permission.Code)
	}
	sort.Strings(codes)
	return codes
}

func userPermissionCodes(user model.User) []string {
	seen := map[string]struct{}{}
	for _, role := range user.Roles {
		for _, permission := range role.Permissions {
			seen[permission.Code] = struct{}{}
		}
	}
	codes := make([]string, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func TestRunMigratesSystemManagementPermissions(t *testing.T) {
	db := openSeedTestDB(t)

	legacyPermissions := []model.Permission{
		{Code: "rbac.read", Name: "RBAC read", Description: "legacy"},
		{Code: "users.write", Name: "Users write", Description: "legacy"},
		{Code: "api_tokens.delete", Name: "API Token delete", Description: "legacy"},
		{Code: "audit.read", Name: "Audit read", Description: "legacy"},
	}
	if err := db.Create(&legacyPermissions).Error; err != nil {
		t.Fatalf("create legacy permissions failed: %v", err)
	}

	role := model.Role{Name: "legacy-system-role", Description: "legacy permissions"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role failed: %v", err)
	}
	if err := db.Model(&role).Association("Permissions").Replace(&legacyPermissions); err != nil {
		t.Fatalf("assign legacy permissions failed: %v", err)
	}

	creator := model.User{Username: "legacy_creator", Name: "Legacy Creator", Email: "legacy_creator@example.com", Password: "hashed", IsActive: true}
	serviceAccount := model.User{Username: "legacy_service", Name: "Legacy Service", Email: "legacy_service@example.com", Password: "hashed", IsActive: true}
	if err := db.Create(&creator).Error; err != nil {
		t.Fatalf("create creator failed: %v", err)
	}
	if err := db.Create(&serviceAccount).Error; err != nil {
		t.Fatalf("create service account failed: %v", err)
	}
	apiToken := model.APIToken{
		Name:             "Legacy Token",
		TokenPrefix:      "pm_legacy",
		TokenLastFour:    "0000",
		TokenHash:        "legacy-token-hash",
		PermissionCodes:  []string{"rbac.read", "users.write", "api_tokens.delete", "audit.read"},
		IsEnabled:        true,
		CreatedByID:      creator.ID,
		ServiceAccountID: serviceAccount.ID,
		ServiceRoleID:    role.ID,
	}
	if err := db.Create(&apiToken).Error; err != nil {
		t.Fatalf("create api token failed: %v", err)
	}

	if err := Run(db); err != nil {
		t.Fatalf("seed run failed: %v", err)
	}

	var legacyCount int64
	if err := db.Model(&model.Permission{}).
		Where("code IN ?", []string{"rbac.read", "users.write", "api_tokens.delete", "audit.read"}).
		Count(&legacyCount).Error; err != nil {
		t.Fatalf("count legacy permissions failed: %v", err)
	}
	if legacyCount != 0 {
		t.Fatalf("expected legacy permissions to be removed, got %d", legacyCount)
	}

	var migratedRole model.Role
	if err := db.Preload("Permissions").First(&migratedRole, role.ID).Error; err != nil {
		t.Fatalf("load migrated role failed: %v", err)
	}
	expectedRoleCodes := []string{
		"system.api_tokens.delete",
		"system.audit.read",
		"system.rbac.read",
		"system.users.create",
		"system.users.delete",
		"system.users.read",
		"system.users.update",
	}
	if got := permissionCodes(migratedRole.Permissions); !reflect.DeepEqual(got, expectedRoleCodes) {
		t.Fatalf("unexpected migrated role permissions: got %#v want %#v", got, expectedRoleCodes)
	}

	var migratedToken model.APIToken
	if err := db.First(&migratedToken, apiToken.ID).Error; err != nil {
		t.Fatalf("load migrated api token failed: %v", err)
	}
	expectedTokenCodes := []string{
		"system.api_tokens.delete",
		"system.audit.read",
		"system.rbac.read",
		"system.users.create",
		"system.users.delete",
		"system.users.read",
		"system.users.update",
	}
	sort.Strings(migratedToken.PermissionCodes)
	if !reflect.DeepEqual(migratedToken.PermissionCodes, expectedTokenCodes) {
		t.Fatalf("unexpected migrated api token permissions: got %#v want %#v", migratedToken.PermissionCodes, expectedTokenCodes)
	}
}

func TestRunSeedsAdminUserWithAllPermissions(t *testing.T) {
	db := openSeedTestDB(t)

	customPermission := model.Permission{Code: "custom.manage", Name: "Custom Manage", Description: "custom permission"}
	if err := db.Create(&customPermission).Error; err != nil {
		t.Fatalf("create custom permission failed: %v", err)
	}

	if err := Run(db); err != nil {
		t.Fatalf("seed run failed: %v", err)
	}
	if err := Run(db); err != nil {
		t.Fatalf("second seed run failed: %v", err)
	}

	var allPermissions []model.Permission
	if err := db.Find(&allPermissions).Error; err != nil {
		t.Fatalf("load all permissions failed: %v", err)
	}
	expectedCodes := permissionCodes(allPermissions)

	var admin model.User
	if err := db.Preload("Roles.Permissions").Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("load admin user failed: %v", err)
	}

	hasAdminRole := false
	for _, role := range admin.Roles {
		if role.Name == "admin" {
			hasAdminRole = true
			break
		}
	}
	if !hasAdminRole {
		t.Fatalf("expected default admin user to have admin role")
	}
	if got := userPermissionCodes(admin); !reflect.DeepEqual(got, expectedCodes) {
		t.Fatalf("unexpected admin permissions: got %#v want %#v", got, expectedCodes)
	}
}

func TestRunDoesNotLogExpectedMissingPermissions(t *testing.T) {
	var logs bytes.Buffer
	db := openSeedTestDBWithConfig(t, &gorm.Config{
		Logger: logger.New(log.New(&logs, "", 0), logger.Config{
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: false,
			Colorful:                  false,
		}),
	})

	if err := Run(db); err != nil {
		t.Fatalf("seed run failed: %v", err)
	}

	if strings.Contains(logs.String(), "record not found") {
		t.Fatalf("seed should not log expected missing permissions, got logs:\n%s", logs.String())
	}
}
