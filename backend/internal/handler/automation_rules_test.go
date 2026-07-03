package handler

import (
	"strings"
	"testing"

	"project-manager/backend/internal/model"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newDryRunMySQLDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8&parseTime=True&loc=Local",
		SkipInitializeWithVersion: true,
	}), &gorm.Config{DisableAutomaticPing: true, DryRun: true})
	if err != nil {
		t.Fatalf("open dry-run mysql db failed: %v", err)
	}
	return db
}

func TestAutomationTriggerWhereQuotesTriggerColumnForMySQL(t *testing.T) {
	db := newDryRunMySQLDB(t)

	var rules []model.AutomationRule
	stmt := whereEnabledAutomationRuleTrigger(db.Model(&model.AutomationRule{}), model.AutomationTriggerTaskOverdue).
		Find(&rules).
		Statement
	sql := stmt.SQL.String()

	if !strings.Contains(sql, "`trigger` = ?") {
		t.Fatalf("expected trigger column to be quoted for MySQL, got SQL: %s", sql)
	}
	if strings.Contains(sql, " trigger = ?") {
		t.Fatalf("expected no unquoted trigger column in MySQL SQL, got: %s", sql)
	}
}
