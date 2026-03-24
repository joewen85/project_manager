package handler

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestParsePage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/?page=2&pageSize=20", nil)

	page, pageSize := parsePage(ctx)
	if page != 2 || pageSize != 20 {
		t.Fatalf("expected page=2 pageSize=20, got page=%d pageSize=%d", page, pageSize)
	}
}

func TestParsePageDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("GET", "/", nil)

	page, pageSize := parsePage(ctx)
	if page != 1 || pageSize != 10 {
		t.Fatalf("expected default page=1 pageSize=10, got page=%d pageSize=%d", page, pageSize)
	}
}

func TestParseRFC3339(t *testing.T) {
	value := "2026-03-24T12:00:00Z"
	parsed, err := parseRFC3339(value)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if parsed == nil || !parsed.Equal(time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected parsed time: %v", parsed)
	}
}

func TestParseRFC3339Invalid(t *testing.T) {
	parsed, err := parseRFC3339("2026-03-24")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if parsed != nil {
		t.Fatalf("expected nil parsed, got %v", parsed)
	}
}
