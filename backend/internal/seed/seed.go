package seed

import (
	"project-manager/backend/internal/model"
	"project-manager/backend/internal/util"

	"gorm.io/gorm"
)

var permissionCodes = []string{
	"rbac.manage",
	"users.read", "users.write",
	"departments.read", "departments.write",
	"projects.read", "projects.write",
	"tasks.read", "tasks.write",
	"stats.read",
	"audit.read",
}

func Run(db *gorm.DB) error {
	for _, code := range permissionCodes {
		perm := model.Permission{Code: code, Name: code}
		if err := db.Where("code = ?", code).FirstOrCreate(&perm).Error; err != nil {
			return err
		}
	}

	var perms []model.Permission
	if err := db.Find(&perms).Error; err != nil {
		return err
	}

	adminRole := model.Role{Name: "admin", Description: "系统管理员"}
	if err := db.Where("name = ?", adminRole.Name).FirstOrCreate(&adminRole).Error; err != nil {
		return err
	}
	if err := db.Model(&adminRole).Association("Permissions").Replace(&perms); err != nil {
		return err
	}

	memberRole := model.Role{Name: "member", Description: "普通成员"}
	if err := db.Where("name = ?", memberRole.Name).FirstOrCreate(&memberRole).Error; err != nil {
		return err
	}

	password, err := util.HashPassword("admin123")
	if err != nil {
		return err
	}

	admin := model.User{Username: "admin", Name: "管理员", Email: "admin@example.com", Password: password, IsActive: true}
	if err := db.Where("username = ?", "admin").FirstOrCreate(&admin).Error; err != nil {
		return err
	}

	if err := db.Model(&admin).Association("Roles").Append(&adminRole); err != nil {
		return err
	}

	return nil
}
