package database

import (
	"project-manager/backend/internal/config"
	"project-manager/backend/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Connect(cfg config.Config) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{})
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.User{},
		&model.Role{},
		&model.Permission{},
		&model.APIToken{},
		&model.Department{},
		&model.Tag{},
		&model.Project{},
		&model.ProjectBaseline{},
		&model.ProjectTemplate{},
		&model.SavedReport{},
		&model.Sprint{},
		&model.SprintTask{},
		&model.WebhookSubscription{},
		&model.WebhookDelivery{},
		&model.Task{},
		&model.TaskDependency{},
		&model.TaskComment{},
		&model.TaskActivity{},
		&model.ProjectRegister{},
		&model.ProjectRegisterActivity{},
		&model.WorkRequest{},
		&model.AutomationRule{},
		&model.AutomationExecutionLog{},
		&model.AuditLog{},
		&model.Notification{},
	)
}
