package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/app"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/config"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"github.com/gin-gonic/gin"
)

func TestRequestLogFormatterIncludesRequestID(t *testing.T) {
	line := requestLogFormatter(gin.LogFormatterParams{
		TimeStamp:  time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		StatusCode: 200,
		Latency:    10 * time.Millisecond,
		ClientIP:   "127.0.0.1",
		Method:     "GET",
		Path:       "/health",
		BodySize:   32,
		Keys: map[string]any{
			middleware.RequestIDContextKey: "request-123",
		},
	})

	if !strings.Contains(line, `request_id="request-123"`) {
		t.Fatalf("log line does not include request id: %s", line)
	}
}

func TestRequestLogFormatterDropsQueryString(t *testing.T) {
	line := requestLogFormatter(gin.LogFormatterParams{
		TimeStamp:  time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		StatusCode: 200,
		Latency:    10 * time.Millisecond,
		ClientIP:   "127.0.0.1",
		Method:     "GET",
		Path:       "/api/v1/reports/summary?start=2026-06-01&token=secret",
	})

	if !strings.Contains(line, `path="/api/v1/reports/summary"`) {
		t.Fatalf("log line does not include sanitized path: %s", line)
	}
	if strings.Contains(line, "token=secret") || strings.Contains(line, "?start=") {
		t.Fatalf("log line leaked query string: %s", line)
	}
}

func TestNewHTTPServerAppliesRuntimeConfig(t *testing.T) {
	cfg := config.Config{
		Port: "19090",
		HTTP: config.HTTPConfig{
			ReadTimeout:       11 * time.Second,
			ReadHeaderTimeout: 3 * time.Second,
			WriteTimeout:      22 * time.Second,
			IdleTimeout:       44 * time.Second,
			MaxHeaderBytes:    32768,
		},
	}

	server := newHTTPServer(cfg, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	if server.Addr != ":19090" {
		t.Fatalf("Addr = %q", server.Addr)
	}
	if server.ReadTimeout != 11*time.Second ||
		server.ReadHeaderTimeout != 3*time.Second ||
		server.WriteTimeout != 22*time.Second ||
		server.IdleTimeout != 44*time.Second ||
		server.MaxHeaderBytes != 32768 {
		t.Fatalf("server timeouts = %#v", server)
	}
}

func TestValidateStartupConfig(t *testing.T) {
	valid := config.Config{
		Port: "8080",
		Database: config.DatabaseConfig{
			Driver:          "postgres",
			MaxOpenConns:    25,
			MaxIdleConns:    10,
			ConnMaxLifetime: time.Hour,
			ConnMaxIdleTime: 30 * time.Minute,
		},
		Redis: config.RedisConfig{Addr: "127.0.0.1:6379", DB: 0},
		HTTP: config.HTTPConfig{
			GinMode:           gin.ReleaseMode,
			CORSAllowOrigins:  []string{"https://app.example.com"},
			ReadTimeout:       15 * time.Second,
			ReadHeaderTimeout: 5 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
			ShutdownTimeout:   10 * time.Second,
			MaxHeaderBytes:    1 << 20,
			MaxBodyBytes:      6 * 1024 * 1024,
		},
		LoginRateLimit: config.LoginRateLimitConfig{MaxFailures: 5, Window: 10 * time.Minute},
		Admin:          config.AdminConfig{Username: "admin", Password: "secret-password"},
		JWT:            config.JWTConfig{Secret: "jwt-secret-with-at-least-32-characters", TTL: time.Hour, Issuer: "issuer", Audience: "api"},
	}
	if err := validateStartupConfig(valid); err != nil {
		t.Fatalf("valid startup config error: %v", err)
	}

	invalidPort := valid
	invalidPort.Port = "70000"
	if err := validateStartupConfig(invalidPort); err == nil {
		t.Fatal("expected invalid port error")
	}

	missingAdmin := valid
	missingAdmin.Admin.Username = ""
	if err := validateStartupConfig(missingAdmin); err == nil {
		t.Fatal("expected missing admin username error")
	}

	invalidCORS := valid
	invalidCORS.HTTP.CORSAllowOrigins = []string{"app.example.com"}
	if err := validateStartupConfig(invalidCORS); err == nil {
		t.Fatal("expected invalid cors origin error")
	}

	insecureRedis := valid
	insecureRedis.Redis.Addr = "redis:6379"
	insecureRedis.Redis.Password = ""
	if err := validateStartupConfig(insecureRedis); err == nil {
		t.Fatal("expected remote redis password error")
	}
}

func TestApplyGinMode(t *testing.T) {
	previousMode := gin.Mode()
	t.Cleanup(func() { gin.SetMode(previousMode) })

	applyGinMode(config.Config{HTTP: config.HTTPConfig{GinMode: gin.ReleaseMode}})
	if gin.Mode() != gin.ReleaseMode {
		t.Fatalf("gin.Mode = %q", gin.Mode())
	}

	applyGinMode(config.Config{HTTP: config.HTTPConfig{GinMode: gin.TestMode}})
	if gin.Mode() != gin.TestMode {
		t.Fatalf("gin.Mode = %q", gin.Mode())
	}
}

func TestNewCORSConfigIncludesRequestIDHeader(t *testing.T) {
	cfg := config.Config{
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: []string{"https://app.example.com"},
		},
	}

	corsConfig := newCORSConfig(cfg)

	if !slices.Contains(corsConfig.AllowHeaders, middleware.RequestIDHeader) {
		t.Fatalf("AllowHeaders = %#v, missing %s", corsConfig.AllowHeaders, middleware.RequestIDHeader)
	}
	if !slices.Contains(corsConfig.ExposeHeaders, middleware.RequestIDHeader) {
		t.Fatalf("ExposeHeaders = %#v, missing %s", corsConfig.ExposeHeaders, middleware.RequestIDHeader)
	}
	if !slices.Contains(corsConfig.ExposeHeaders, "Link") || !slices.Contains(corsConfig.ExposeHeaders, "X-Total-Count") {
		t.Fatalf("ExposeHeaders = %#v, missing pagination headers", corsConfig.ExposeHeaders)
	}
	if !slices.Contains(corsConfig.ExposeHeaders, "WWW-Authenticate") {
		t.Fatalf("ExposeHeaders = %#v, missing WWW-Authenticate", corsConfig.ExposeHeaders)
	}
	if !slices.Contains(corsConfig.ExposeHeaders, "Retry-After") {
		t.Fatalf("ExposeHeaders = %#v, missing Retry-After", corsConfig.ExposeHeaders)
	}
	if !slices.Contains(corsConfig.ExposeHeaders, "Allow") {
		t.Fatalf("ExposeHeaders = %#v, missing Allow", corsConfig.ExposeHeaders)
	}
}

func TestValidateCORSConfig(t *testing.T) {
	valid := config.Config{
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: []string{"https://app.example.com", "http://localhost:3000"},
		},
	}
	if err := validateCORSConfig(valid); err != nil {
		t.Fatalf("valid cors config error: %v", err)
	}

	for _, origins := range [][]string{
		{"app.example.com"},
		{"https://app.example.com/api"},
		{"https://*.example.com"},
		{"chrome-extension://abc"},
		{},
	} {
		t.Run(strings.Join(origins, ","), func(t *testing.T) {
			cfg := config.Config{HTTP: config.HTTPConfig{CORSAllowOrigins: origins}}
			if err := validateCORSConfig(cfg); err == nil {
				t.Fatalf("expected invalid cors config for %#v", origins)
			}
		})
	}
}

func TestSecurityHeadersConfigUsesHTTPConfig(t *testing.T) {
	cfg := config.Config{
		HTTP: config.HTTPConfig{
			HSTSMaxAgeSeconds:     31536000,
			HSTSIncludeSubDomains: true,
			HSTSPreload:           true,
		},
	}

	securityConfig := securityHeadersConfig(cfg)

	if securityConfig.HSTSMaxAgeSeconds != 31536000 ||
		!securityConfig.HSTSIncludeSubDomains ||
		!securityConfig.HSTSPreload {
		t.Fatalf("security headers config = %#v", securityConfig)
	}
}

func TestLoggerConfigSkipsHealthProbePaths(t *testing.T) {
	previousMode := gin.Mode()
	previousWriter := gin.DefaultWriter
	t.Cleanup(func() {
		gin.SetMode(previousMode)
		gin.DefaultWriter = previousWriter
	})
	gin.SetMode(gin.TestMode)

	var logOutput bytes.Buffer
	gin.DefaultWriter = &logOutput
	router := gin.New()
	router.Use(gin.LoggerWithConfig(newLoggerConfig()))
	router.GET("/livez", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/api/v1/accounts", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/livez", nil))
	if got := logOutput.String(); got != "" {
		t.Fatalf("health probe log = %q", got)
	}

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil))
	if got := logOutput.String(); !strings.Contains(got, `path="/api/v1/accounts"`) {
		t.Fatalf("api log = %q", got)
	}
}

func TestLoggerConfigDoesNotLogQueryString(t *testing.T) {
	previousMode := gin.Mode()
	previousWriter := gin.DefaultWriter
	t.Cleanup(func() {
		gin.SetMode(previousMode)
		gin.DefaultWriter = previousWriter
	})
	gin.SetMode(gin.TestMode)

	var logOutput bytes.Buffer
	gin.DefaultWriter = &logOutput
	router := gin.New()
	router.Use(gin.LoggerWithConfig(newLoggerConfig()))
	router.GET("/api/v1/reports/summary", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary?start=2026-06-01&token=secret", nil))

	got := logOutput.String()
	if !strings.Contains(got, `path="/api/v1/reports/summary"`) {
		t.Fatalf("api log = %q", got)
	}
	if strings.Contains(got, "token=secret") || strings.Contains(got, "?start=") {
		t.Fatalf("api log leaked query string: %q", got)
	}
}

func TestLoggerConfigSkipsOnlyProbePaths(t *testing.T) {
	config := newLoggerConfig()

	for _, path := range []string{"/livez", "/readyz", "/health"} {
		if !slices.Contains(config.SkipPaths, path) {
			t.Fatalf("SkipPaths = %#v, missing %s", config.SkipPaths, path)
		}
	}
	if config.Formatter == nil {
		t.Fatal("expected formatter")
	}
}

func TestEarlyRejectedRequestsKeepGlobalHeaders(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	cfg := config.Config{
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: []string{"https://app.example.com"},
			MaxBodyBytes:     4,
		},
	}
	router := gin.New()
	applyGlobalMiddleware(router, cfg)
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	tests := []struct {
		name        string
		path        string
		body        string
		contentType string
		accept      string
		wantStatus  int
	}{
		{name: "body limit", path: "/api/v1/auth/login", body: "12345", contentType: "application/json", wantStatus: http.StatusRequestEntityTooLarge},
		{name: "content type", path: "/api/v1/auth/login", body: "{}", contentType: "text/plain", wantStatus: http.StatusUnsupportedMediaType},
		{name: "accept", path: "/api/v1/auth/login", body: "{}", contentType: "application/json", accept: "text/csv", wantStatus: http.StatusNotAcceptable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Origin", "https://app.example.com")
			req.Header.Set("Content-Type", tt.contentType)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}
			req.Header.Set(middleware.RequestIDHeader, "request-123")
			resp := httptest.NewRecorder()

			router.ServeHTTP(resp, req)

			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", resp.Code, tt.wantStatus, resp.Body.String())
			}
			if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
				t.Fatalf("Access-Control-Allow-Origin = %q", got)
			}
			if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("X-Content-Type-Options = %q", got)
			}
			if got := resp.Header().Get(middleware.RequestIDHeader); got != "request-123" {
				t.Fatalf("%s = %q", middleware.RequestIDHeader, got)
			}
			if tt.wantStatus == http.StatusNotAcceptable {
				if got := resp.Header().Get("Vary"); got != "Origin, Accept" {
					t.Fatalf("Vary = %q", got)
				}
			}
		})
	}
}

func TestCORSPreflightKeepsGlobalMiddlewareHeaders(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	cfg := config.Config{
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: []string{"https://app.example.com"},
			MaxBodyBytes:     6 * 1024 * 1024,
		},
	}
	router := gin.New()
	applyGlobalMiddleware(router, cfg)
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "Authorization, X-Request-ID, Content-Type")
	req.Header.Set(middleware.RequestIDHeader, "request-preflight")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", resp.Code, http.StatusNoContent, resp.Body.String())
	}
	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := resp.Header().Get("Access-Control-Max-Age"); got != "43200" {
		t.Fatalf("Access-Control-Max-Age = %q", got)
	}
	if got := resp.Header().Get("Access-Control-Allow-Methods"); !headerHasToken(got, http.MethodPost) {
		t.Fatalf("Access-Control-Allow-Methods = %q, missing %s", got, http.MethodPost)
	}
	allowHeaders := resp.Header().Get("Access-Control-Allow-Headers")
	for _, header := range []string{"Authorization", middleware.RequestIDHeader, "Content-Type"} {
		if !headerHasToken(allowHeaders, header) {
			t.Fatalf("Access-Control-Allow-Headers = %q, missing %s", allowHeaders, header)
		}
	}
	if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := resp.Header().Get(middleware.RequestIDHeader); got != "request-preflight" {
		t.Fatalf("%s = %q", middleware.RequestIDHeader, got)
	}
}

func headerHasToken(value, token string) bool {
	for _, part := range strings.Split(value, ",") {
		if strings.EqualFold(strings.TrimSpace(part), token) {
			return true
		}
	}
	return false
}

func TestFallbackResponsesKeepGlobalMiddlewareHeaders(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	cfg := config.Config{
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: []string{"https://app.example.com"},
			MaxBodyBytes:     6 * 1024 * 1024,
		},
		Admin:          config.AdminConfig{Username: "admin", Password: "secret-password", Name: "管理员"},
		LoginRateLimit: config.LoginRateLimitConfig{MaxFailures: 5, Window: time.Minute},
		JWT:            config.JWTConfig{Secret: "test-jwt-secret-with-at-least-32-chars", TTL: time.Hour, Issuer: "issuer", Audience: "api"},
	}
	router := gin.New()
	applyGlobalMiddleware(router, cfg)
	if err := app.RegisterRoutesWithConfig(router, testutil.NewStore(t), nil, cfg); err != nil {
		t.Fatalf("register routes: %v", err)
	}

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantCode   string
	}{
		{name: "not found", method: http.MethodGet, path: "/api/v1/missing", wantStatus: http.StatusNotFound, wantCode: httputil.CodeNotFound},
		{name: "method not allowed", method: http.MethodPatch, path: "/api/v1/accounts", wantStatus: http.StatusMethodNotAllowed, wantCode: httputil.CodeMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			req.Header.Set("Origin", "https://app.example.com")
			req.Header.Set(middleware.RequestIDHeader, "request-boundary")
			resp := httptest.NewRecorder()

			router.ServeHTTP(resp, req)

			if resp.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body = %s", resp.Code, tt.wantStatus, resp.Body.String())
			}
			if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
				t.Fatalf("Access-Control-Allow-Origin = %q", got)
			}
			if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("X-Content-Type-Options = %q", got)
			}
			if got := resp.Header().Get(middleware.RequestIDHeader); got != "request-boundary" {
				t.Fatalf("%s = %q", middleware.RequestIDHeader, got)
			}
			if got := resp.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q", got)
			}

			var body httputil.ErrorResponse
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
				t.Fatalf("parse body: %v, body = %s", err, resp.Body.String())
			}
			if body.Code != tt.wantCode || body.RequestID != "request-boundary" {
				t.Fatalf("body = %#v", body)
			}
		})
	}
}

func TestRunHTTPServerStopsWhenContextIsCanceled(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	server := &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- runHTTPServer(ctx, server, time.Second)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, err := http.Get("http://" + addr)
		if err == nil {
			if err := resp.Body.Close(); err != nil {
				t.Fatal(err)
			}
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("server did not start: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runHTTPServer returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runHTTPServer did not stop after context cancellation")
	}
}
