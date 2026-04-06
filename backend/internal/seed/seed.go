package seed

import (
	"errors"

	"project-manager/backend/internal/model"
	"project-manager/backend/internal/util"

	"gorm.io/gorm"
)

type permissionSeed struct {
	Code        string
	Name        string
	Description string
}

type legacyPermissionMapping struct {
	LegacyCode  string
	TargetCodes []string
}

var permissionCatalog = []permissionSeed{
	{Code: "rbac.create", Name: "RBAC-创建", Description: "创建角色与权限"},
	{Code: "rbac.read", Name: "RBAC-查看", Description: "查看角色与权限"},
	{Code: "rbac.update", Name: "RBAC-更新", Description: "更新角色与权限"},
	{Code: "rbac.delete", Name: "RBAC-删除", Description: "删除角色与权限"},

	{Code: "users.create", Name: "用户-创建", Description: "创建用户"},
	{Code: "users.read", Name: "用户-查看", Description: "查看用户"},
	{Code: "users.update", Name: "用户-更新", Description: "更新用户"},
	{Code: "users.delete", Name: "用户-删除", Description: "删除用户"},

	{Code: "departments.create", Name: "部门-创建", Description: "创建部门"},
	{Code: "departments.read", Name: "部门-查看", Description: "查看部门"},
	{Code: "departments.update", Name: "部门-更新", Description: "更新部门"},
	{Code: "departments.delete", Name: "部门-删除", Description: "删除部门"},

	{Code: "projects.create", Name: "项目-创建", Description: "创建项目"},
	{Code: "projects.read", Name: "项目-查看", Description: "查看项目"},
	{Code: "projects.update", Name: "项目-更新", Description: "更新项目"},
	{Code: "projects.delete", Name: "项目-删除", Description: "删除项目"},

	{Code: "tasks.create", Name: "任务-创建", Description: "创建任务"},
	{Code: "tasks.read", Name: "任务-查看", Description: "查看任务"},
	{Code: "tasks.update", Name: "任务-更新", Description: "更新任务"},
	{Code: "tasks.delete", Name: "任务-删除", Description: "删除任务"},

	{Code: "tags.create", Name: "标签-创建", Description: "创建标签"},
	{Code: "tags.read", Name: "标签-查看", Description: "查看标签"},
	{Code: "tags.update", Name: "标签-更新", Description: "更新标签"},
	{Code: "tags.delete", Name: "标签-删除", Description: "删除标签"},

	{Code: "notifications.read", Name: "通知-查看", Description: "查看通知与未读数"},
	{Code: "notifications.update", Name: "通知-更新", Description: "标记通知已读"},

	{Code: "stats.read", Name: "统计-查看", Description: "查看统计分析"},
	{Code: "audit.read", Name: "审计-查看", Description: "查看审计日志"},
	{Code: "uploads.create", Name: "上传-创建", Description: "上传附件"},
}

var legacyPermissionMappings = []legacyPermissionMapping{
	{LegacyCode: "rbac.manage", TargetCodes: []string{"rbac.create", "rbac.read", "rbac.update", "rbac.delete"}},
	{LegacyCode: "users.write", TargetCodes: []string{"users.create", "users.read", "users.update", "users.delete"}},
	{LegacyCode: "departments.write", TargetCodes: []string{"departments.create", "departments.read", "departments.update", "departments.delete"}},
	{LegacyCode: "projects.write", TargetCodes: []string{"projects.create", "projects.read", "projects.update", "projects.delete"}},
	{LegacyCode: "tasks.write", TargetCodes: []string{"tasks.create", "tasks.read", "tasks.update", "tasks.delete"}},
	{LegacyCode: "tags.write", TargetCodes: []string{"tags.create", "tags.read", "tags.update", "tags.delete"}},
	{LegacyCode: "notifications.write", TargetCodes: []string{"notifications.read", "notifications.update"}},
}

func upsertPermissionCatalog(tx *gorm.DB) error {
	for _, entry := range permissionCatalog {
		perm := model.Permission{}
		err := tx.Where("code = ?", entry.Code).First(&perm).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			perm = model.Permission{
				Code:        entry.Code,
				Name:        entry.Name,
				Description: entry.Description,
			}
			if createErr := tx.Create(&perm).Error; createErr != nil {
				return createErr
			}
			continue
		}
		if err != nil {
			return err
		}
		if updateErr := tx.Model(&perm).Updates(map[string]any{
			"name":        entry.Name,
			"description": entry.Description,
		}).Error; updateErr != nil {
			return updateErr
		}
	}
	return nil
}

func migrateLegacyPermissionBindings(tx *gorm.DB) error {
	for _, mapping := range legacyPermissionMappings {
		legacyPerm := model.Permission{}
		err := tx.Where("code = ?", mapping.LegacyCode).First(&legacyPerm).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return err
		}

		targetPerms := make([]model.Permission, 0, len(mapping.TargetCodes))
		if findErr := tx.Where("code IN ?", mapping.TargetCodes).Find(&targetPerms).Error; findErr != nil {
			return findErr
		}

		roleIDs := make([]uint, 0)
		if pluckErr := tx.Table("role_permissions").
			Where("permission_id = ?", legacyPerm.ID).
			Distinct("role_id").
			Pluck("role_id", &roleIDs).Error; pluckErr != nil {
			return pluckErr
		}

		if len(targetPerms) > 0 {
			for _, roleID := range roleIDs {
				role := model.Role{}
				role.ID = roleID
				if appendErr := tx.Model(&role).Association("Permissions").Append(&targetPerms); appendErr != nil {
					return appendErr
				}
			}
		}

		if deleteAssocErr := tx.Exec("DELETE FROM role_permissions WHERE permission_id = ?", legacyPerm.ID).Error; deleteAssocErr != nil {
			return deleteAssocErr
		}
		if deletePermErr := tx.Delete(&legacyPerm).Error; deletePermErr != nil {
			return deletePermErr
		}
	}
	return nil
}

func Run(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := upsertPermissionCatalog(tx); err != nil {
			return err
		}
		if err := migrateLegacyPermissionBindings(tx); err != nil {
			return err
		}

		var perms []model.Permission
		if err := tx.Find(&perms).Error; err != nil {
			return err
		}

		adminRole := model.Role{Name: "admin", Description: "系统管理员"}
		if err := tx.Where("name = ?", adminRole.Name).FirstOrCreate(&adminRole).Error; err != nil {
			return err
		}
		if err := tx.Model(&adminRole).Association("Permissions").Replace(&perms); err != nil {
			return err
		}

		memberRole := model.Role{Name: "member", Description: "普通成员"}
		if err := tx.Where("name = ?", memberRole.Name).FirstOrCreate(&memberRole).Error; err != nil {
			return err
		}
		var memberDefaultPerms []model.Permission
		if err := tx.Where("code IN ?", []string{"notifications.read", "notifications.update"}).Find(&memberDefaultPerms).Error; err != nil {
			return err
		}
		if len(memberDefaultPerms) > 0 {
			if err := tx.Model(&memberRole).Association("Permissions").Append(&memberDefaultPerms); err != nil {
				return err
			}
		}

		password, err := util.HashPassword("admin123")
		if err != nil {
			return err
		}

		admin := model.User{Username: "admin", Name: "管理员", Email: "admin@example.com", Password: password, IsActive: true}
		if err := tx.Where("username = ?", "admin").FirstOrCreate(&admin).Error; err != nil {
			return err
		}
		if err := tx.Model(&admin).Association("Roles").Append(&adminRole); err != nil {
			return err
		}

		return nil
	})
}
