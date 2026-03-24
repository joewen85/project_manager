package handler

import (
	"project-manager/backend/internal/config"

	"gorm.io/gorm"
)

type Handler struct {
	DB  *gorm.DB
	Cfg config.Config
}

func New(db *gorm.DB, cfg config.Config) *Handler {
	return &Handler{DB: db, Cfg: cfg}
}
