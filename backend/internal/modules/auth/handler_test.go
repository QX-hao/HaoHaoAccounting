package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestEnsureBootstrapAdminCreatesHashedPasswordUser(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "secret-password")
	t.Setenv("ADMIN_NAME", "管理员")

	s := testutil.NewStore(t)
	if err := NewHandler(s).EnsureBootstrapAdmin(); err != nil {
		t.Fatalf("bootstrap admin: %v", err)
	}

	var user models.User
	if err := s.DB.Where("username = ?", "admin").First(&user).Error; err != nil {
		t.Fatalf("load admin: %v", err)
	}
	if user.PasswordHash == "" || user.PasswordHash == "secret-password" {
		t.Fatalf("password was not hashed: %q", user.PasswordHash)
	}
	if !verifyPassword(user.PasswordHash, "secret-password") {
		t.Fatal("hashed password does not verify")
	}
}

func TestLoginRateLimiterBlocksRepeatedFailures(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-jwt-secret-with-at-least-32-chars")
	gin.SetMode(gin.TestMode)

	s := testutil.NewStore(t)
	hash, err := hashPassword("secret-password")
	if err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "admin", PasswordHash: hash, Name: "管理员"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	handler := &Handler{store: s, loginLimiter: newLoginLimiter(2, time.Minute)}
	handler.RegisterPublic(router.Group("/api/v1"))

	for i := 0; i < 2; i++ {
		resp := postLogin(t, router, `{"username":"admin","password":"wrong"}`)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401", i+1, resp.Code)
		}
	}

	resp := postLogin(t, router, `{"username":"admin","password":"wrong"}`)
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked status = %d, want 429", resp.Code)
	}
}

func TestLoginRateLimiterClearsAfterSuccessfulLogin(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-jwt-secret-with-at-least-32-chars")
	gin.SetMode(gin.TestMode)

	s := testutil.NewStore(t)
	hash, err := hashPassword("secret-password")
	if err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "admin", PasswordHash: hash, Name: "管理员"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	limiter := newLoginLimiter(2, time.Minute)
	handler := &Handler{store: s, loginLimiter: limiter}
	handler.RegisterPublic(router.Group("/api/v1"))

	if resp := postLogin(t, router, `{"username":"admin","password":"wrong"}`); resp.Code != http.StatusUnauthorized {
		t.Fatalf("first failure status = %d, want 401", resp.Code)
	}
	if resp := postLogin(t, router, `{"username":"admin","password":"secret-password"}`); resp.Code != http.StatusOK {
		t.Fatalf("success status = %d, want 200", resp.Code)
	}
	if !limiter.Allow(loginLimiterKey("192.0.2.1", "admin")) {
		t.Fatal("expected successful login to clear limiter")
	}
}

func postLogin(t *testing.T, router http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.1:1234"
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}
