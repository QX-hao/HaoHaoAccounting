package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/app"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/config"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
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

func TestRequestLogFormatterIncludesProtocolAndUserAgent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	req.Header.Set("User-Agent", "HaoHaoMobile/1.0")

	line := requestLogFormatter(gin.LogFormatterParams{
		Request:    req,
		TimeStamp:  time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		StatusCode: 200,
		Latency:    10 * time.Millisecond,
		ClientIP:   "127.0.0.1",
		Method:     "GET",
		Path:       "/api/v1/accounts",
	})

	for _, want := range []string{`proto="HTTP/1.1"`, `user_agent="HaoHaoMobile/1.0"`} {
		if !strings.Contains(line, want) {
			t.Fatalf("log line = %q, missing %s", line, want)
		}
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

func TestReadmeDocumentsStartupAndMiddlewareContracts(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"`LoadStrict`",
		"`validateStartupConfig`",
		"`CORS_ALLOW_ORIGINS`",
		"explicit `http` or `https` origins",
		"wildcards",
		"credentials disabled",
		"`gin-contrib/cors`",
		"queued resource locations",
		"`TRUSTED_PROXIES`",
		"`RequestID` -> `HTTPMetrics` -> `RequestTimeout` -> logger -> `Recovery` -> `SecurityHeaders` -> CORS -> `NoStoreAPI` -> `BodyLimit` -> `ContentType` -> `Accept`",
		"no-store API cache headers",
		"being counted by request metrics",
		"per-request",
		"`HTTP_REQUEST_TIMEOUT` defaults to `60s`",
		"`0s`",
		"access log records `time`, `status`, `latency`, `client_ip`, `method`, sanitized `path`, `proto`, `user_agent`, `request_id`, response `bytes`, and `error`",
		"`/metrics`",
		"method, Gin route pattern, and status",
		"early rejections",
		"`X-Request-ID`",
		"`HTTP_*`",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("README.md is missing server guidance %q", want)
		}
	}
}

func TestMetricsEndpointExportsHTTPMetrics(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	router := gin.New()
	registry := prometheus.NewRegistry()
	registerMetricsRoute(router, registry)
	applyGlobalMiddleware(router, config.Config{HTTP: config.HTTPConfig{
		CORSAllowOrigins: []string{"https://app.example.com"},
	}}, middleware.NewHTTPMetrics(registry))
	router.GET("/api/v1/accounts/:id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/42?token=secret", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsResp := httptest.NewRecorder()
	router.ServeHTTP(metricsResp, metricsReq)

	if metricsResp.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, body = %s", metricsResp.Code, metricsResp.Body.String())
	}
	body := metricsResp.Body.String()
	for _, want := range []string{
		`haohao_http_requests_total{method="GET",route="/api/v1/accounts/:id",status="204"} 1`,
		`haohao_http_request_duration_seconds_bucket{method="GET",route="/api/v1/accounts/:id",status="204"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "/api/v1/accounts/42") || strings.Contains(body, "token=secret") {
		t.Fatalf("metrics leaked raw URL data: %s", body)
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
	for _, header := range []string{"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset"} {
		if !slices.Contains(corsConfig.ExposeHeaders, header) {
			t.Fatalf("ExposeHeaders = %#v, missing %s", corsConfig.ExposeHeaders, header)
		}
	}
	if !slices.Contains(corsConfig.ExposeHeaders, "Allow") {
		t.Fatalf("ExposeHeaders = %#v, missing Allow", corsConfig.ExposeHeaders)
	}
	if !slices.Contains(corsConfig.ExposeHeaders, "Location") {
		t.Fatalf("ExposeHeaders = %#v, missing Location", corsConfig.ExposeHeaders)
	}
}

func TestNewCORSConfigKeepsExplicitOriginsAndNoCredentials(t *testing.T) {
	cfg := config.Config{
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: []string{"https://app.example.com"},
		},
	}

	corsConfig := newCORSConfig(cfg)

	if corsConfig.AllowAllOrigins {
		t.Fatal("AllowAllOrigins must stay disabled")
	}
	if corsConfig.AllowCredentials {
		t.Fatal("AllowCredentials must stay disabled")
	}
	if got := corsConfig.AllowOrigins; !slices.Equal(got, []string{"https://app.example.com"}) {
		t.Fatalf("AllowOrigins = %#v", got)
	}
}

func TestCORSAllowMethodsCoverRegisteredAPIMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := config.Config{
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: []string{"https://app.example.com"},
		},
	}
	router := gin.New()
	if err := app.RegisterRoutesWithConfig(router, testutil.NewStore(t), nil, corsRouteContractConfig()); err != nil {
		t.Fatalf("register routes: %v", err)
	}

	corsConfig := newCORSConfig(cfg)
	for _, route := range router.Routes() {
		if !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}
		if !slices.Contains(corsConfig.AllowMethods, route.Method) {
			t.Fatalf("AllowMethods = %#v, missing %s for %s", corsConfig.AllowMethods, route.Method, route.Path)
		}
	}
}

func corsRouteContractConfig() config.Config {
	return config.Config{
		Admin: config.AdminConfig{
			Username: "admin",
			Password: "secret-password",
			Name:     "管理员",
		},
		LoginRateLimit: config.LoginRateLimitConfig{MaxFailures: 5, Window: time.Minute},
		JWT: config.JWTConfig{
			Secret:    "test-jwt-secret-with-at-least-32-chars",
			TTL:       time.Hour,
			ClockSkew: 30 * time.Second,
			Issuer:    "issuer",
			Audience:  "api",
		},
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
		{"https://app.example.com:bad"},
		{"https:///app.example.com"},
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

func TestLoggerConfigIncludesStructuredErrorSummary(t *testing.T) {
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
	router.GET("/api/v1/accounts", func(c *gin.Context) {
		httputil.BadRequest(c, "raw user input: password=secret")
	})

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil))

	got := logOutput.String()
	if !strings.Contains(got, `error="Error #01: status=400 code=bad_request`) {
		t.Fatalf("api log = %q, missing structured error summary", got)
	}
	if strings.Contains(got, "password=secret") || strings.Contains(got, "raw user input") {
		t.Fatalf("api log leaked response message: %q", got)
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
	applyGlobalMiddleware(router, cfg, middleware.NewHTTPMetrics(prometheus.NewRegistry()))
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
			if got := resp.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("Cache-Control = %q", got)
			}
			if got := resp.Header().Get("Pragma"); got != "no-cache" {
				t.Fatalf("Pragma = %q", got)
			}
			if got := resp.Header().Get("Expires"); got != "0" {
				t.Fatalf("Expires = %q", got)
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
	router := newCORSMiddlewareTestRouter(t)
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/login", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "Authorization, X-Request-ID, Content-Type, X-Admin-Override")
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
	vary := strings.Join(resp.Header().Values("Vary"), ",")
	for _, header := range []string{"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers"} {
		if !headerHasToken(vary, header) {
			t.Fatalf("Vary = %q, missing %s", vary, header)
		}
	}
	allowHeaders := resp.Header().Get("Access-Control-Allow-Headers")
	for _, header := range []string{"Authorization", middleware.RequestIDHeader, "Content-Type"} {
		if !headerHasToken(allowHeaders, header) {
			t.Fatalf("Access-Control-Allow-Headers = %q, missing %s", allowHeaders, header)
		}
	}
	if headerHasToken(allowHeaders, "X-Admin-Override") {
		t.Fatalf("Access-Control-Allow-Headers = %q, should not allow X-Admin-Override", allowHeaders)
	}
	if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := resp.Header().Get(middleware.RequestIDHeader); got != "request-preflight" {
		t.Fatalf("%s = %q", middleware.RequestIDHeader, got)
	}
}

func TestCORSAllowedOriginExposesClientHeaders(t *testing.T) {
	router := newCORSMiddlewareTestRouter(t)
	router.GET("/api/v1/accounts", func(c *gin.Context) {
		c.Header("Link", "</api/v1/accounts?page=2>; rel=\"next\"")
		c.Header("X-Total-Count", "2")
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set(middleware.RequestIDHeader, "request-cors-expose")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", resp.Code, http.StatusNoContent, resp.Body.String())
	}
	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := resp.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("Access-Control-Allow-Credentials = %q, want empty", got)
	}
	exposeHeaders := resp.Header().Get("Access-Control-Expose-Headers")
	for _, header := range []string{"Allow", "Content-Disposition", "Link", "Location", "RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset", "WWW-Authenticate", "Retry-After", "X-Total-Count", middleware.RequestIDHeader} {
		if !headerHasToken(exposeHeaders, header) {
			t.Fatalf("Access-Control-Expose-Headers = %q, missing %s", exposeHeaders, header)
		}
	}
	if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := resp.Header().Get(middleware.RequestIDHeader); got != "request-cors-expose" {
		t.Fatalf("%s = %q", middleware.RequestIDHeader, got)
	}
}

func TestCORSIgnoresSameOriginRequestsWithOriginHeader(t *testing.T) {
	router := newCORSMiddlewareTestRouter(t)
	router.GET("/api/v1/me", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "https://api.example.com/api/v1/me", nil)
	req.Host = "api.example.com"
	req.Header.Set("Origin", "https://api.example.com")
	req.Header.Set(middleware.RequestIDHeader, "request-same-origin")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", resp.Code, http.StatusNoContent, resp.Body.String())
	}
	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
	if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := resp.Header().Get(middleware.RequestIDHeader); got != "request-same-origin" {
		t.Fatalf("%s = %q", middleware.RequestIDHeader, got)
	}
}

func TestCORSIgnoresRequestsWithoutOriginHeader(t *testing.T) {
	router := newCORSMiddlewareTestRouter(t)
	router.GET("/api/v1/me", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set(middleware.RequestIDHeader, "request-no-origin")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", resp.Code, http.StatusNoContent, resp.Body.String())
	}
	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
	if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := resp.Header().Get(middleware.RequestIDHeader); got != "request-no-origin" {
		t.Fatalf("%s = %q", middleware.RequestIDHeader, got)
	}
}

func TestCORSRejectsUntrustedOriginsWithoutAllowHeaders(t *testing.T) {
	router := newCORSMiddlewareTestRouter(t)
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	tests := []struct {
		name       string
		method     string
		preflight  bool
		wantAbsent []string
	}{
		{
			name:      "actual request",
			method:    http.MethodPost,
			preflight: false,
			wantAbsent: []string{
				"Access-Control-Allow-Origin",
				"Access-Control-Allow-Credentials",
				"Access-Control-Expose-Headers",
			},
		},
		{
			name:      "preflight",
			method:    http.MethodOptions,
			preflight: true,
			wantAbsent: []string{
				"Access-Control-Allow-Origin",
				"Access-Control-Allow-Credentials",
				"Access-Control-Allow-Methods",
				"Access-Control-Allow-Headers",
				"Access-Control-Max-Age",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/auth/login", nil)
			req.Header.Set("Origin", "https://evil.example.com")
			req.Header.Set(middleware.RequestIDHeader, "request-denied-cors")
			if tt.preflight {
				req.Header.Set("Access-Control-Request-Method", http.MethodPost)
				req.Header.Set("Access-Control-Request-Headers", "Authorization, X-Request-ID, Content-Type")
			}
			resp := httptest.NewRecorder()

			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d, body = %s", resp.Code, http.StatusForbidden, resp.Body.String())
			}
			for _, header := range tt.wantAbsent {
				if got := resp.Header().Get(header); got != "" {
					t.Fatalf("%s = %q, want empty", header, got)
				}
			}
			if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
				t.Fatalf("X-Content-Type-Options = %q", got)
			}
			if got := resp.Header().Get(middleware.RequestIDHeader); got != "request-denied-cors" {
				t.Fatalf("%s = %q", middleware.RequestIDHeader, got)
			}
		})
	}
}

func newCORSMiddlewareTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
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
	applyGlobalMiddleware(router, cfg, middleware.NewHTTPMetrics(prometheus.NewRegistry()))
	return router
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
	applyGlobalMiddleware(router, cfg, middleware.NewHTTPMetrics(prometheus.NewRegistry()))
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

func TestRunHTTPServerReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	server := &http.Server{Addr: listener.Addr().String()}
	err = runHTTPServer(context.Background(), server, time.Second)
	if err == nil {
		t.Fatal("expected listen error")
	}
	if !errors.Is(err, syscall.EADDRINUSE) {
		t.Fatalf("listen error = %v", err)
	}
}

func TestRunHTTPServerTreatsExternalCloseAsCleanShutdown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	server := &http.Server{Addr: addr}
	errCh := make(chan error, 1)
	go func() {
		errCh <- runHTTPServer(context.Background(), server, time.Second)
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
			_ = server.Close()
			t.Fatalf("server did not start: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := server.Close(); err != nil {
		t.Fatalf("close server: %v", err)
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runHTTPServer returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runHTTPServer did not stop after server close")
	}
}
