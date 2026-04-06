package handler

import (
	"testing"
	"time"

	"project-manager/backend/internal/database"
	"project-manager/backend/internal/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupAuditTestHandler(t *testing.T) *Handler {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err = database.Migrate(db); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return &Handler{DB: db}
}

func createAuditLogWithCreatedAt(t *testing.T, db *gorm.DB, createdAt time.Time) {
	t.Helper()
	entry := model.AuditLog{
		UserID:    1,
		Module:    "tests",
		Action:    "create",
		TargetID:  1,
		Method:    "POST",
		Path:      "/api/v1/tests",
		Success:   true,
		Detail:    "test entry",
		ClientIP:  "127.0.0.1",
		UserAgent: "go-test",
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("create audit log failed: %v", err)
	}
	if err := db.Model(&model.AuditLog{}).Where("id = ?", entry.ID).Update("created_at", createdAt).Error; err != nil {
		t.Fatalf("update audit log created_at failed: %v", err)
	}
}

func TestDeleteExpiredAuditLogs(t *testing.T) {
	h := setupAuditTestHandler(t)

	now := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	createAuditLogWithCreatedAt(t, h.DB, now.AddDate(0, -7, 0))
	createAuditLogWithCreatedAt(t, h.DB, now.AddDate(0, -3, 0))

	deletedCount, err := h.DeleteExpiredAuditLogs(now)
	if err != nil {
		t.Fatalf("DeleteExpiredAuditLogs failed: %v", err)
	}
	if deletedCount != 1 {
		t.Fatalf("expected deleted count 1, got %d", deletedCount)
	}

	var remain int64
	if err = h.DB.Model(&model.AuditLog{}).Count(&remain).Error; err != nil {
		t.Fatalf("count remaining logs failed: %v", err)
	}
	if remain != 1 {
		t.Fatalf("expected remaining logs 1, got %d", remain)
	}
}

func TestAuditRetentionCutoff(t *testing.T) {
	now := time.Date(2026, 4, 6, 10, 0, 0, 0, time.UTC)
	cutoff := auditRetentionCutoff(now)
	want := time.Date(2025, 10, 6, 10, 0, 0, 0, time.UTC)
	if !cutoff.Equal(want) {
		t.Fatalf("unexpected cutoff: want %s got %s", want, cutoff)
	}
}
