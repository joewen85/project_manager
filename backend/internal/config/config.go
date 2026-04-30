package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

const defaultJWTSecret = "change-me"

type Config struct {
	AppName            string
	Port               string
	JWTSecret          string
	CORSAllowOrigins   string
	UploadDir          string
	UploadPublicBase   string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPassword         string
	DBName             string
	TaskNotifyProvider string
	SMTPHost           string
	SMTPPort           string
	SMTPUsername       string
	SMTPPassword       string
	SMTPFrom           string
	WeComCorpID        string
	WeComCorpSecret    string
	WeComAgentID       string
	WeComToUser        string
	DingTalkWebhook    string
	DingTalkSecret     string
	FeishuAppID        string
	FeishuAppSecret    string
	FeishuReceiveID    string
	FeishuReceiveType  string
}

func Load() Config {
	cfg := Config{
		AppName:            getEnv("APP_NAME", "project-manager"),
		Port:               getEnv("APP_PORT", "8080"),
		JWTSecret:          getEnv("JWT_SECRET", defaultJWTSecret),
		CORSAllowOrigins:   getEnv("CORS_ALLOW_ORIGINS", "http://localhost:5173"),
		UploadDir:          getEnv("UPLOAD_DIR", "./static/uploads"),
		UploadPublicBase:   getEnv("UPLOAD_PUBLIC_BASE", "/static/uploads"),
		DBHost:             getEnv("DB_HOST", "127.0.0.1"),
		DBPort:             getEnv("DB_PORT", "3306"),
		DBUser:             getEnv("DB_USER", "root"),
		DBPassword:         getEnv("DB_PASSWORD", "root"),
		DBName:             getEnv("DB_NAME", "project_management"),
		TaskNotifyProvider: getEnv("TASK_NOTIFY_PROVIDER", ""),
		SMTPHost:           getEnv("SMTP_HOST", ""),
		SMTPPort:           getEnv("SMTP_PORT", "25"),
		SMTPUsername:       getEnv("SMTP_USERNAME", ""),
		SMTPPassword:       getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:           getEnv("SMTP_FROM", ""),
		WeComCorpID:        getEnv("WECOM_CORP_ID", ""),
		WeComCorpSecret:    getEnv("WECOM_CORP_SECRET", ""),
		WeComAgentID:       getEnv("WECOM_AGENT_ID", ""),
		WeComToUser:        getEnv("WECOM_TO_USER", "@all"),
		DingTalkWebhook:    getEnv("DINGTALK_WEBHOOK", ""),
		DingTalkSecret:     getEnv("DINGTALK_SECRET", ""),
		FeishuAppID:        getEnv("FEISHU_APP_ID", ""),
		FeishuAppSecret:    getEnv("FEISHU_APP_SECRET", ""),
		FeishuReceiveID:    getEnv("FEISHU_RECEIVE_ID", ""),
		FeishuReceiveType:  getEnv("FEISHU_RECEIVE_ID_TYPE", "email"),
	}
	return cfg
}

func (c Config) Validate() error {
	secret := strings.TrimSpace(c.JWTSecret)
	if secret == "" {
		return errors.New("JWT_SECRET is required")
	}
	if secret == defaultJWTSecret || strings.EqualFold(secret, "change-me-in-production") {
		return errors.New("JWT_SECRET uses an insecure default value")
	}
	return nil
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
