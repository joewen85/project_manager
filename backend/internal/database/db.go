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
		&model.Department{},
		&model.Project{},
		&model.Task{},
		&model.AuditLog{},
	)
}
