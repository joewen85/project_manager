package handler

import (
	"net/http/httptest"
	"testing"

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
