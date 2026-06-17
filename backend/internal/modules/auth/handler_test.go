package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/config"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/models"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestEnsureBootstrapAdminCreatesHashedPasswordUser(t *testing.T) {
	s := testutil.NewStore(t)
	tokenService := testTokenService(t)
	handler := NewHandlerWithConfig(s, config.AdminConfig{
		Username: "admin",
		Password: "secret-password",
		Name:     "管理员",
	}, config.LoginRateLimitConfig{MaxFailures: 5, Window: time.Minute}, tokenService)

	if err := handler.EnsureBootstrapAdmin(); err != nil {
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
	handler := &Handler{store: s, loginLimiter: newLoginLimiter(2, time.Minute), tokenService: testTokenService(t)}
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
	if got := resp.Header().Get("Retry-After"); got != "60" {
		t.Fatalf("Retry-After = %q", got)
	}
}

func TestLoginRateLimiterReportsRemainingRetryAfter(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-jwt-secret-with-at-least-32-chars")
	gin.SetMode(gin.TestMode)

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
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
	limiter.now = func() time.Time { return now }
	handler := &Handler{store: s, loginLimiter: limiter, tokenService: testTokenService(t)}
	handler.RegisterPublic(router.Group("/api/v1"))

	for i := 0; i < 2; i++ {
		resp := postLogin(t, router, `{"username":"admin","password":"wrong"}`)
		if resp.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, want 401", i+1, resp.Code)
		}
	}

	now = now.Add(45 * time.Second)
	resp := postLogin(t, router, `{"username":"admin","password":"wrong"}`)
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("blocked status = %d, want 429", resp.Code)
	}
	if got := resp.Header().Get("Retry-After"); got != "15" {
		t.Fatalf("Retry-After = %q", got)
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
	handler := &Handler{store: s, loginLimiter: limiter, tokenService: testTokenService(t)}
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

func TestLoginPreservesPasswordWhitespace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := testutil.NewStore(t)
	hash, err := hashPassword(" secret-password ")
	if err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "admin", PasswordHash: hash, Name: "管理员"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	handler := &Handler{store: s, loginLimiter: newLoginLimiter(3, time.Minute), tokenService: testTokenService(t)}
	handler.RegisterPublic(router.Group("/api/v1"))

	if resp := postLogin(t, router, `{"username":"admin","password":"secret-password"}`); resp.Code != http.StatusUnauthorized {
		t.Fatalf("trimmed password status = %d, want 401", resp.Code)
	}
	if resp := postLogin(t, router, `{"username":" admin ","password":" secret-password "}`); resp.Code != http.StatusOK {
		t.Fatalf("exact password status = %d, want 200, body = %s", resp.Code, resp.Body.String())
	}
}

func TestLoginRateLimiterPrunesExpiredAttempts(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	limiter := newLoginLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("192.0.2.1|admin")
	if len(limiter.attempts) != 1 {
		t.Fatalf("attempts = %d, want 1", len(limiter.attempts))
	}

	now = now.Add(2 * time.Minute)
	if !limiter.Allow("192.0.2.2|other") {
		t.Fatal("new key should be allowed")
	}
	if len(limiter.attempts) != 0 {
		t.Fatalf("attempts = %d, want expired attempts pruned", len(limiter.attempts))
	}
}

func TestLoginRateLimiterExpiresAttemptsAtWindowBoundary(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	limiter := newLoginLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("192.0.2.1|admin")
	limiter.RecordFailure("192.0.2.1|admin")
	if limiter.Allow("192.0.2.1|admin") {
		t.Fatal("expected attempts to be blocked before window boundary")
	}

	now = now.Add(time.Minute)
	if !limiter.Allow("192.0.2.1|admin") {
		t.Fatal("expected attempts to expire at window boundary")
	}
	if len(limiter.attempts) != 0 {
		t.Fatalf("attempts = %d, want expired attempts pruned", len(limiter.attempts))
	}
}

func TestLoginRateLimiterKeepsFreshAttemptsWhenPruning(t *testing.T) {
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	limiter := newLoginLimiter(2, time.Minute)
	limiter.now = func() time.Time { return now }

	limiter.RecordFailure("192.0.2.1|admin")
	now = now.Add(30 * time.Second)
	limiter.RecordFailure("192.0.2.2|other")

	if len(limiter.attempts) != 2 {
		t.Fatalf("attempts = %d, want fresh attempts kept", len(limiter.attempts))
	}
}

func TestLoginReturnsCurrentUserContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := testutil.NewStore(t)
	hash, err := hashPassword("secret-password")
	if err != nil {
		t.Fatal(err)
	}
	user := models.User{
		Username:     "admin",
		PasswordHash: hash,
		Name:         "管理员",
		Phone:        "13800000000",
		Email:        "admin@example.com",
		WechatID:     "haohao",
	}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	handler := &Handler{store: s, loginLimiter: newLoginLimiter(2, time.Minute), tokenService: testTokenService(t)}
	handler.RegisterPublic(router.Group("/api/v1"))

	resp := postLogin(t, router, `{"username":"admin","password":"secret-password"}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", resp.Code, resp.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["token"].(string); !ok {
		t.Fatalf("token missing or not a string: %#v", body["token"])
	}
	userBody, ok := body["user"].(map[string]any)
	if !ok {
		t.Fatalf("user missing or not an object: %#v", body["user"])
	}
	assertOnlyKeys(t, userBody, "id", "name", "username", "phone", "email", "wechatId")
	if userBody["username"] != "admin" || userBody["phone"] != "13800000000" || userBody["email"] != "admin@example.com" || userBody["wechatId"] != "haohao" {
		t.Fatalf("unexpected user body: %#v", userBody)
	}
}

func TestMeReturnsCurrentUserContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := testutil.NewStore(t)
	user := models.User{Username: "admin", PasswordHash: "hash", Name: "管理员"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", user.ID)
		c.Next()
	})
	handler := &Handler{store: s, tokenService: testTokenService(t)}
	handler.RegisterPrivate(router.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", resp.Code, resp.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	assertOnlyKeys(t, body, "id", "name", "username")
	if body["username"] != "admin" || body["name"] != "管理员" {
		t.Fatalf("unexpected user body: %#v", body)
	}
}

func TestRefreshRevokesCurrentTokenWhenRevokerIsConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := testutil.NewStore(t)
	user := models.User{Username: "admin", PasswordHash: "hash", Name: "管理员"}
	if err := s.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	tokenService := testTokenService(t)
	token, err := tokenService.BuildToken(user.ID)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	revocationStore := &recordingRevocationStore{enabled: true}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", user.ID)
		c.Set("token_revoker", &TokenRevoker{cache: revocationStore})
		c.Next()
	})
	handler := &Handler{store: s, tokenService: tokenService}
	handler.RegisterPrivate(router.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", resp.Code, resp.Body.String())
	}
	if revocationStore.calls != 1 {
		t.Fatalf("revocation calls = %d, want 1", revocationStore.calls)
	}
	if revocationStore.key == "" || revocationStore.value != "1" || revocationStore.ttl <= 0 {
		t.Fatalf("revocation store = %#v", revocationStore)
	}
}

func TestLoginReturnsPayloadTooLargeWhenBodyLimitIsExceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.RequestID(), middleware.BodyLimit(8))
	handler := &Handler{store: testutil.NewStore(t), loginLimiter: newLoginLimiter(2, time.Minute), tokenService: testTokenService(t)}
	handler.RegisterPublic(router.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"secret-password"}`))
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.RequestIDHeader, "request-789")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get(middleware.RequestIDHeader); got != "request-789" {
		t.Fatalf("request id header = %q", got)
	}
}

func TestTokenRevocationTTLIncludesClockSkew(t *testing.T) {
	tokenService, err := middleware.NewTokenServiceWithTTL("test-jwt-secret-with-at-least-32-chars", time.Hour, 30*time.Second, "haohao-accounting", "haohao-accounting-api")
	if err != nil {
		t.Fatalf("token service: %v", err)
	}
	handler := &Handler{tokenService: tokenService}

	token, err := tokenService.BuildTokenWithTTL(42, 30*time.Minute)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	ttl, err := handler.tokenRevocationTTL(token)
	if err != nil {
		t.Fatalf("token revocation ttl: %v", err)
	}
	if ttl < 30*time.Minute || ttl > 31*time.Minute {
		t.Fatalf("ttl = %s, want close to token lifetime plus clock skew", ttl)
	}
}

func TestLogoutReturnsUnauthorizedWhenRevocationTTLCannotBeComputed(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("token_revoker", &TokenRevoker{cache: &recordingRevocationStore{enabled: true}})
		c.Next()
	})
	handler := &Handler{store: testutil.NewStore(t), tokenService: testTokenService(t)}
	handler.RegisterPrivate(router.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", resp.Code, resp.Body.String())
	}
}

func TestTokenRevokerStoresRevocationWithProvidedTTL(t *testing.T) {
	store := &recordingRevocationStore{enabled: true}
	revoker := &TokenRevoker{cache: store}

	if err := revoker.RevokeToken(context.Background(), "token-value", 30*time.Minute); err != nil {
		t.Fatalf("revoke token: %v", err)
	}

	if store.key == "" || store.value != "1" {
		t.Fatalf("store = %#v", store)
	}
	if store.ttl != 30*time.Minute {
		t.Fatalf("ttl = %s", store.ttl)
	}
}

func TestTokenRevokerSkipsExpiredTTL(t *testing.T) {
	store := &recordingRevocationStore{enabled: true}
	revoker := &TokenRevoker{cache: store}

	if err := revoker.RevokeToken(context.Background(), "token-value", 0); err != nil {
		t.Fatalf("revoke token: %v", err)
	}
	if store.calls != 0 {
		t.Fatalf("store calls = %d, want 0", store.calls)
	}
}

type recordingRevocationStore struct {
	enabled bool
	key     string
	value   string
	ttl     time.Duration
	calls   int
}

func (s *recordingRevocationStore) Enabled() bool {
	return s.enabled
}

func (s *recordingRevocationStore) Exists(context.Context, string) (bool, error) {
	return false, nil
}

func (s *recordingRevocationStore) SetString(_ context.Context, key, value string, ttl time.Duration) error {
	s.calls++
	s.key = key
	s.value = value
	s.ttl = ttl
	return nil
}

func testTokenService(t *testing.T) *middleware.TokenService {
	t.Helper()
	tokenService, err := middleware.NewTokenService("test-jwt-secret-with-at-least-32-chars")
	if err != nil {
		t.Fatalf("token service: %v", err)
	}
	return tokenService
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

func assertOnlyKeys(t *testing.T, value map[string]any, keys ...string) {
	t.Helper()
	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
		if _, ok := value[key]; !ok {
			t.Fatalf("missing key %q in %#v", key, value)
		}
	}
	for key := range value {
		if _, ok := allowed[key]; !ok {
			t.Fatalf("unexpected key %q in %#v", key, value)
		}
	}
}
