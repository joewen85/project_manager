package main

import (
	"log"
	"time"

	"project-manager/backend/internal/config"
	"project-manager/backend/internal/database"
	"project-manager/backend/internal/handler"
	"project-manager/backend/internal/router"
	"project-manager/backend/internal/seed"

	"github.com/joho/godotenv"
)

func startAuditRetentionJob(h *handler.Handler) {
	cleanup := func(trigger string) {
		deletedCount, err := h.DeleteExpiredAuditLogs(time.Now())
		if err != nil {
			log.Printf("audit retention cleanup failed(%s): %v", trigger, err)
			return
		}
		if deletedCount > 0 {
			log.Printf("audit retention cleanup deleted %d expired logs", deletedCount)
		}
	}

	cleanup("startup")

	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for range ticker.C {
			cleanup("scheduled")
		}
	}()
}

func main() {
	_ = godotenv.Load(".env")
	_ = godotenv.Load("../.env")

	cfg := config.Load()
	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}

	if err = database.Migrate(db); err != nil {
		log.Fatalf("db migrate failed: %v", err)
	}

	if err = seed.Run(db); err != nil {
		log.Fatalf("db seed failed: %v", err)
	}

	h := handler.New(db, cfg)
	startAuditRetentionJob(h)
	r := router.New(cfg, h)

	if err = r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server run failed: %v", err)
	}
}
