package handler

import (
	"project-manager/backend/internal/config"

	"gorm.io/gorm"
)

type Handler struct {
	DB          *gorm.DB
	Cfg         config.Config
	TxFailpoint func(point string) error
}

func New(db *gorm.DB, cfg config.Config) *Handler {
	return &Handler{DB: db, Cfg: cfg}
}

func (h *Handler) triggerFailpoint(point string) error {
	if h.TxFailpoint == nil {
		return nil
	}
	return h.TxFailpoint(point)
}
