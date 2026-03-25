package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"project-manager/backend/internal/auth"

	"github.com/gin-gonic/gin"
)

func TestJWTUnauthorizedWithoutToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/secure", JWT("secret"), func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestJWTAndPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, err := auth.GenerateToken("secret", 1, "admin", []string{"tasks.read"})
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}

	r := gin.New()
	r.GET("/tasks", JWT("secret"), RequirePermission(nil, "tasks.read"), func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestPermissionForbidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, err := auth.GenerateToken("secret", 1, "user", []string{"users.read"})
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}

	r := gin.New()
	r.GET("/tasks", JWT("secret"), RequirePermission(nil, "tasks.read"), func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestWritePermissionImpliesRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, err := auth.GenerateToken("secret", 1, "user", []string{"projects.write"})
	if err != nil {
		t.Fatalf("generate token failed: %v", err)
	}

	r := gin.New()
	r.GET("/projects", JWT("secret"), RequirePermission(nil, "projects.read"), func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestResolveEffectivePermissionsCachesInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	claims := &auth.Claims{Permissions: []string{"tasks.read"}}

	first, err := resolveEffectivePermissions(c, nil, claims)
	if err != nil {
		t.Fatalf("resolve effective permissions failed: %v", err)
	}
	if len(first) != 1 || first[0] != "tasks.read" {
		t.Fatalf("unexpected permissions: %#v", first)
	}

	claims.Permissions = []string{"users.read"}
	second, err := resolveEffectivePermissions(c, nil, claims)
	if err != nil {
		t.Fatalf("resolve effective permissions second call failed: %v", err)
	}
	if len(second) != 1 || second[0] != "tasks.read" {
		t.Fatalf("expected cached permissions, got %#v", second)
	}
}
