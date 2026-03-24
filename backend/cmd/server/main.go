package main

import (
	"log"

	"project-manager/backend/internal/config"
	"project-manager/backend/internal/database"
	"project-manager/backend/internal/handler"
	"project-manager/backend/internal/router"
	"project-manager/backend/internal/seed"

	"github.com/joho/godotenv"
)

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
	r := router.New(cfg, h)

	if err = r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server run failed: %v", err)
	}
}
