package config

import (
	"fmt"
	"os"
)

type Config struct {
	AppName          string
	Port             string
	JWTSecret        string
	CORSAllowOrigins string
	DBHost           string
	DBPort           string
	DBUser           string
	DBPassword       string
	DBName           string
}

func Load() Config {
	cfg := Config{
		AppName:          getEnv("APP_NAME", "project-manager"),
		Port:             getEnv("APP_PORT", "8080"),
		JWTSecret:        getEnv("JWT_SECRET", "change-me"),
		CORSAllowOrigins: getEnv("CORS_ALLOW_ORIGINS", "http://localhost:5173"),
		DBHost:           getEnv("DB_HOST", "127.0.0.1"),
		DBPort:           getEnv("DB_PORT", "3306"),
		DBUser:           getEnv("DB_USER", "root"),
		DBPassword:       getEnv("DB_PASSWORD", "root"),
		DBName:           getEnv("DB_NAME", "project_management"),
	}
	return cfg
}

func (c Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
