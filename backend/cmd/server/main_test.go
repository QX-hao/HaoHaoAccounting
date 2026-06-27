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
	"path/filepath"
	"slices"
	"strings"
	"sync"
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
	"gopkg.in/yaml.v3"
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

func TestRequestLogFormatterRejectsUnsafeRequestID(t *testing.T) {
	for _, requestID := range []string{
		strings.Repeat("a", 129),
		"bad\nid",
		"bad id",
	} {
		t.Run(requestID, func(t *testing.T) {
			line := requestLogFormatter(gin.LogFormatterParams{
				TimeStamp:  time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
				StatusCode: 200,
				Latency:    10 * time.Millisecond,
				ClientIP:   "127.0.0.1",
				Method:     "GET",
				Path:       "/health",
				BodySize:   32,
				Keys: map[string]any{
					middleware.RequestIDContextKey: requestID,
				},
			})

			if !strings.Contains(line, `request_id="-"`) {
				t.Fatalf("log line = %q, want unsafe request id placeholder", line)
			}
			if strings.Contains(line, requestID) {
				t.Fatalf("log line leaked unsafe request id: %q", line)
			}
		})
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

func TestRequestLogFormatterUsesRootForEmptyPath(t *testing.T) {
	for _, path := range []string{"", "?token=secret"} {
		t.Run(path, func(t *testing.T) {
			line := requestLogFormatter(gin.LogFormatterParams{
				TimeStamp:  time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
				StatusCode: 200,
				Latency:    10 * time.Millisecond,
				ClientIP:   "127.0.0.1",
				Method:     "GET",
				Path:       path,
			})

			if !strings.Contains(line, `path="/"`) {
				t.Fatalf("log line = %q, missing root path placeholder", line)
			}
			if strings.Contains(line, "token=secret") {
				t.Fatalf("log line leaked query string: %q", line)
			}
		})
	}
}

func TestRequestLogFormatterEscapesClientControlledFields(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	req.Header.Set("User-Agent", "HaoHao\nMobile")

	line := requestLogFormatter(gin.LogFormatterParams{
		Request:      req,
		TimeStamp:    time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		StatusCode:   500,
		Latency:      10 * time.Millisecond,
		ClientIP:     "127.0.0.1",
		Method:       "GET",
		Path:         "/api/v1/accounts\nforged",
		ErrorMessage: "db\nfailed",
	})

	for _, want := range []string{`path="/api/v1/accounts\nforged"`, `user_agent="HaoHao\nMobile"`, `error="db\nfailed"`} {
		if !strings.Contains(line, want) {
			t.Fatalf("log line = %q, missing escaped field %s", line, want)
		}
	}
	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Fatalf("log line should stay single-line: %q", line)
	}
}

func TestRequestLogFormatterEscapesMethod(t *testing.T) {
	line := requestLogFormatter(gin.LogFormatterParams{
		TimeStamp:  time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		StatusCode: 200,
		Latency:    10 * time.Millisecond,
		ClientIP:   "127.0.0.1",
		Method:     "GET\nFORGED",
		Path:       "/api/v1/accounts",
	})

	if !strings.Contains(line, `method="GET\nFORGED"`) {
		t.Fatalf("log line = %q, missing escaped method", line)
	}
	if strings.Count(line, "\n") != 1 || !strings.HasSuffix(line, "\n") {
		t.Fatalf("log line should stay single-line: %q", line)
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

func TestRequestLogFormatterBoundsUserAgentLength(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	req.Header.Set("User-Agent", strings.Repeat("a", maxLoggedUserAgentLength+20))

	line := requestLogFormatter(gin.LogFormatterParams{
		Request:    req,
		TimeStamp:  time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		StatusCode: 200,
		Latency:    10 * time.Millisecond,
		ClientIP:   "127.0.0.1",
		Method:     "GET",
		Path:       "/api/v1/accounts",
	})

	want := strings.Repeat("a", maxLoggedUserAgentLength-3) + "..."
	if !strings.Contains(line, `user_agent="`+want+`"`) {
		t.Fatalf("log line = %q, missing truncated user agent", line)
	}
	if strings.Contains(line, strings.Repeat("a", maxLoggedUserAgentLength+1)) {
		t.Fatalf("log line contains unbounded user agent: %q", line)
	}
}

func TestRequestLogFormatterTruncatesUserAgentOnCharacterBoundary(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil)
	req.Header.Set("User-Agent", strings.Repeat("好", maxLoggedUserAgentLength+20))

	line := requestLogFormatter(gin.LogFormatterParams{
		Request:    req,
		TimeStamp:  time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		StatusCode: 200,
		Latency:    10 * time.Millisecond,
		ClientIP:   "127.0.0.1",
		Method:     "GET",
		Path:       "/api/v1/accounts",
	})

	want := strings.Repeat("好", maxLoggedUserAgentLength-3) + "..."
	if !strings.Contains(line, `user_agent="`+want+`"`) {
		t.Fatalf("log line = %q, missing character-safe truncated user agent", line)
	}
}

func TestRequestLogFormatterUsesPlaceholderForMissingUserAgent(t *testing.T) {
	line := requestLogFormatter(gin.LogFormatterParams{
		Request:    httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil),
		TimeStamp:  time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		StatusCode: 200,
		Latency:    10 * time.Millisecond,
		ClientIP:   "127.0.0.1",
		Method:     "GET",
		Path:       "/api/v1/accounts",
	})

	if !strings.Contains(line, `user_agent="-"`) {
		t.Fatalf("log line = %q, missing empty user agent placeholder", line)
	}
}

func TestRequestLogFormatterBoundsErrorLength(t *testing.T) {
	line := requestLogFormatter(gin.LogFormatterParams{
		Request:      httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil),
		TimeStamp:    time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		StatusCode:   500,
		Latency:      10 * time.Millisecond,
		ClientIP:     "127.0.0.1",
		Method:       "GET",
		Path:         "/api/v1/accounts",
		ErrorMessage: strings.Repeat("a", maxLoggedErrorLength+20),
	})

	want := strings.Repeat("a", maxLoggedErrorLength-3) + "..."
	if !strings.Contains(line, `error="`+want+`"`) {
		t.Fatalf("log line = %q, missing truncated error", line)
	}
	if strings.Contains(line, strings.Repeat("a", maxLoggedErrorLength+1)) {
		t.Fatalf("log line contains unbounded error: %q", line)
	}
}

func TestRequestLogFormatterTruncatesErrorOnCharacterBoundary(t *testing.T) {
	line := requestLogFormatter(gin.LogFormatterParams{
		Request:      httptest.NewRequest(http.MethodGet, "/api/v1/accounts", nil),
		TimeStamp:    time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC),
		StatusCode:   500,
		Latency:      10 * time.Millisecond,
		ClientIP:     "127.0.0.1",
		Method:       "GET",
		Path:         "/api/v1/accounts",
		ErrorMessage: strings.Repeat("好", maxLoggedErrorLength+20),
	})

	want := strings.Repeat("好", maxLoggedErrorLength-3) + "..."
	if !strings.Contains(line, `error="`+want+`"`) {
		t.Fatalf("log line = %q, missing character-safe truncated error", line)
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
		"trimmed, normalized",
		"deduplicated",
		"normalized to browser `Origin` header form",
		"lowercase scheme/host and default ports removed",
		"credentials disabled",
		"`gin-contrib/cors`",
		"queued resource locations",
		"`TRUSTED_PROXIES`",
		"Leave it empty for direct traffic",
		"client-supplied `X-Forwarded-*` headers are ignored",
		"trusted reverse proxy IPs or CIDRs",
		"`RequestID` -> `HTTPMetrics` -> `RequestTimeout` -> logger -> `Recovery` -> `SecurityHeaders` -> CORS -> `NoStoreAPI` -> `BodyLimit` -> `ContentType` -> `Accept`",
		"no-store API cache headers",
		"being counted by request metrics",
		"per-request",
		"`HTTP_REQUEST_TIMEOUT` defaults to `60s`",
		"`0s`",
		"`SecurityHeaders` writes baseline browser security headers",
		"`HTTP_HSTS_MAX_AGE_SECONDS`",
		"`HTTP_HSTS_PRELOAD=true` requires `HTTP_HSTS_INCLUDE_SUBDOMAINS=true`",
		"`HTTP_CROSS_ORIGIN_EMBEDDER_POLICY`",
		"`require-corp`, `credentialless`, or `unsafe-none`",
		"access log records `time`, `status`, `latency`, `client_ip`, `method`, sanitized `path`, `proto`, `user_agent`, `request_id`, response `bytes`, and `error`",
		"`user_agent` values are trimmed to 256 characters before logging",
		"`error` values are trimmed to 512 characters before logging",
		"`HTTP_METRICS_ENABLED=true`",
		"`HTTP_METRICS_TOKEN`",
		"`/metrics`",
		"`promhttp_metric_handler_errors_total`",
		"scrape gathering or encoding failures",
		"in-flight HTTP request gauge",
		"completed HTTP request metrics",
		"`HTTP_METRICS_MAX_REQUESTS_IN_FLIGHT` limits concurrent scrapes",
		"`HTTP_METRICS_TIMEOUT` bounds one scrape duration",
		"`503 Service Unavailable`",
		"Keep it disabled unless the backend port is protected",
		"registered before API middleware",
		"not affected by API `Accept` and `Content-Type` negotiation",
		"still applies `RequestID`, `Recovery`, `SecurityHeaders`, and `NoCache` revalidation headers",
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
	metrics := installMetrics(router, config.Config{HTTP: config.HTTPConfig{MetricsEnabled: true}})
	applyGlobalMiddleware(router, config.Config{HTTP: config.HTTPConfig{
		CORSAllowOrigins: []string{"https://app.example.com"},
	}}, metrics)
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
	secondMetricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	secondMetricsResp := httptest.NewRecorder()
	router.ServeHTTP(secondMetricsResp, secondMetricsReq)
	if secondMetricsResp.Code != http.StatusOK {
		t.Fatalf("second metrics status = %d, body = %s", secondMetricsResp.Code, secondMetricsResp.Body.String())
	}
	if !middleware.ValidRequestID(secondMetricsResp.Header().Get(middleware.RequestIDHeader)) {
		t.Fatalf("metrics response missing valid request id header: %#v", secondMetricsResp.Header())
	}
	assertMetricsNoCacheHeaders(t, secondMetricsResp)
	assertMetricsSecurityHeaders(t, secondMetricsResp)
	body := secondMetricsResp.Body.String()
	for _, want := range []string{
		`haohao_http_requests_total{method="GET",route="/api/v1/accounts/:id",status="204"} 1`,
		`haohao_http_request_duration_seconds_bucket{method="GET",route="/api/v1/accounts/:id",status="204"`,
		`haohao_http_requests_in_flight `,
		`go_goroutines `,
		`process_cpu_seconds_total `,
		`promhttp_metric_handler_requests_total{code="200"} 1`,
		`promhttp_metric_handler_errors_total{cause="gathering"} 0`,
		`promhttp_metric_handler_errors_total{cause="encoding"} 0`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q: %s", want, body)
		}
	}
	if strings.Contains(body, "/api/v1/accounts/42") || strings.Contains(body, "token=secret") {
		t.Fatalf("metrics leaked raw URL data: %s", body)
	}
}

func TestRecoveredPanicsAreCountedByHTTPMetrics(t *testing.T) {
	previousMode := gin.Mode()
	previousWriter := gin.DefaultWriter
	previousErrorWriter := gin.DefaultErrorWriter
	gin.SetMode(gin.TestMode)
	var logOutput bytes.Buffer
	gin.DefaultWriter = &logOutput
	gin.DefaultErrorWriter = &logOutput
	t.Cleanup(func() {
		gin.SetMode(previousMode)
		gin.DefaultWriter = previousWriter
		gin.DefaultErrorWriter = previousErrorWriter
	})

	router := gin.New()
	metrics := installMetrics(router, config.Config{HTTP: config.HTTPConfig{MetricsEnabled: true}})
	applyGlobalMiddleware(router, config.Config{HTTP: config.HTTPConfig{
		CORSAllowOrigins: []string{"https://app.example.com"},
	}}, metrics)
	router.GET("/api/v1/panic", func(*gin.Context) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/panic", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("panic status = %d, body = %s", resp.Code, resp.Body.String())
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsResp := httptest.NewRecorder()
	router.ServeHTTP(metricsResp, metricsReq)
	if metricsResp.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, body = %s", metricsResp.Code, metricsResp.Body.String())
	}

	body := metricsResp.Body.String()
	for _, want := range []string{
		`haohao_http_requests_total{method="GET",route="/api/v1/panic",status="500"} 1`,
		`haohao_http_request_duration_seconds_bucket{method="GET",route="/api/v1/panic",status="500"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q: %s", want, body)
		}
	}
}

func TestMetricsEndpointCanRequireBearerToken(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	router := gin.New()
	installMetrics(router, config.Config{HTTP: config.HTTPConfig{
		MetricsEnabled: true,
		MetricsToken:   "scrape-secret",
	}})

	for _, tc := range []struct {
		name   string
		header string
		status int
	}{
		{name: "missing", status: http.StatusUnauthorized},
		{name: "wrong", header: "Bearer wrong-secret", status: http.StatusUnauthorized},
		{name: "malformed", header: "Basic scrape-secret", status: http.StatusUnauthorized},
		{name: "valid", header: "Bearer scrape-secret", status: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != tc.status {
				t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
			}
			if !middleware.ValidRequestID(resp.Header().Get(middleware.RequestIDHeader)) {
				t.Fatalf("metrics response missing valid request id header: %#v", resp.Header())
			}
			assertMetricsNoCacheHeaders(t, resp)
			assertMetricsSecurityHeaders(t, resp)
		})
	}
}

func TestMetricsEndpointRejectsDuplicateAuthorizationHeaders(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	router := gin.New()
	installMetrics(router, config.Config{HTTP: config.HTTPConfig{
		MetricsEnabled: true,
		MetricsToken:   "scrape-secret",
	}})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Add("Authorization", "Bearer scrape-secret")
	req.Header.Add("Authorization", "Bearer scrape-secret")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if !middleware.ValidRequestID(resp.Header().Get(middleware.RequestIDHeader)) {
		t.Fatalf("metrics response missing valid request id header: %#v", resp.Header())
	}
	assertMetricsNoCacheHeaders(t, resp)
	assertMetricsSecurityHeaders(t, resp)
}

func TestMetricsEndpointLimitsConcurrentScrapes(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	router := gin.New()
	registry := prometheus.NewRegistry()
	registerMetricsRoute(router, registry, config.Config{HTTP: config.HTTPConfig{
		MetricsEnabled:             true,
		MetricsMaxRequestsInFlight: 1,
	}})

	entered := make(chan struct{})
	release := make(chan struct{})
	registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "haohao_test_blocking_metric",
		Help: "Blocks collection so concurrent scrape limiting can be observed.",
	}, func() float64 {
		select {
		case <-entered:
		default:
			close(entered)
		}
		<-release
		return 1
	}))

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		if resp.Code != http.StatusOK {
			t.Errorf("first scrape status = %d, body = %s", resp.Code, resp.Body.String())
		}
	}()

	<-entered
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if resp.Code != http.StatusServiceUnavailable {
		close(release)
		wg.Wait()
		t.Fatalf("second scrape status = %d, body = %s", resp.Code, resp.Body.String())
	}
	close(release)
	wg.Wait()
}

func TestMetricsEndpointTimesOutSlowScrapes(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	router := gin.New()
	registry := prometheus.NewRegistry()
	registerMetricsRoute(router, registry, config.Config{HTTP: config.HTTPConfig{
		MetricsEnabled:             true,
		MetricsMaxRequestsInFlight: 1,
		MetricsTimeout:             time.Nanosecond,
	}})
	registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "haohao_test_slow_metric",
		Help: "Sleeps so scrape timeout behavior can be observed.",
	}, func() float64 {
		time.Sleep(time.Second)
		return 1
	}))

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func assertMetricsNoCacheHeaders(t *testing.T, resp *httptest.ResponseRecorder) {
	t.Helper()
	for header, want := range map[string]string{
		"Cache-Control": "no-cache",
		"Pragma":        "no-cache",
		"Expires":       "0",
	} {
		if got := resp.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
}

func assertMetricsSecurityHeaders(t *testing.T, resp *httptest.ResponseRecorder) {
	t.Helper()
	for header, want := range map[string]string{
		"Content-Security-Policy":    "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'",
		"Cross-Origin-Opener-Policy": "same-origin",
		"Referrer-Policy":            "no-referrer",
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
	} {
		if got := resp.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestConstantTimeTokenEqualMatchesExactTokenOnly(t *testing.T) {
	for _, tc := range []struct {
		name string
		got  string
		want string
		ok   bool
	}{
		{name: "same", got: "scrape-secret", want: "scrape-secret", ok: true},
		{name: "different same length", got: "scrape-secret", want: "scrape-secreu", ok: false},
		{name: "different shorter", got: "short", want: "scrape-secret", ok: false},
		{name: "different longer", got: "scrape-secret-extra", want: "scrape-secret", ok: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := constantTimeTokenEqual(tc.got, tc.want); got != tc.ok {
				t.Fatalf("constantTimeTokenEqual(%q, %q) = %v, want %v", tc.got, tc.want, got, tc.ok)
			}
		})
	}
}

func TestMetricsEndpointIsRegisteredOnlyWhenEnabled(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	disabledRouter := gin.New()
	installMetrics(disabledRouter, config.Config{})
	disabledResp := httptest.NewRecorder()
	disabledRouter.ServeHTTP(disabledResp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if disabledResp.Code != http.StatusNotFound {
		t.Fatalf("disabled metrics status = %d, body = %s", disabledResp.Code, disabledResp.Body.String())
	}

	enabledRouter := gin.New()
	installMetrics(enabledRouter, config.Config{HTTP: config.HTTPConfig{MetricsEnabled: true}})
	enabledResp := httptest.NewRecorder()
	enabledRouter.ServeHTTP(enabledResp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if enabledResp.Code != http.StatusOK {
		t.Fatalf("enabled metrics status = %d, body = %s", enabledResp.Code, enabledResp.Body.String())
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
			GinMode:                    gin.ReleaseMode,
			CORSAllowOrigins:           []string{"https://app.example.com"},
			ReadTimeout:                15 * time.Second,
			ReadHeaderTimeout:          5 * time.Second,
			WriteTimeout:               30 * time.Second,
			IdleTimeout:                60 * time.Second,
			ShutdownTimeout:            10 * time.Second,
			MaxHeaderBytes:             1 << 20,
			MaxBodyBytes:               6 * 1024 * 1024,
			MetricsMaxRequestsInFlight: 1,
			MetricsTimeout:             10 * time.Second,
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

func TestTrustedProxiesControlForwardedClientIP(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	for _, tc := range []struct {
		name           string
		trustedProxies []string
		wantClientIP   string
	}{
		{name: "untrusted direct client", wantClientIP: "198.51.100.10"},
		{name: "trusted reverse proxy", trustedProxies: []string{"198.51.100.10"}, wantClientIP: "203.0.113.42"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := gin.New()
			if err := router.SetTrustedProxies(tc.trustedProxies); err != nil {
				t.Fatalf("SetTrustedProxies: %v", err)
			}
			router.GET("/client-ip", func(c *gin.Context) {
				c.String(http.StatusOK, c.ClientIP())
			})

			req := httptest.NewRequest(http.MethodGet, "/client-ip", nil)
			req.RemoteAddr = "198.51.100.10:12345"
			req.Header.Set("X-Forwarded-For", "203.0.113.42")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
			}
			if got := resp.Body.String(); got != tc.wantClientIP {
				t.Fatalf("ClientIP = %q, want %q", got, tc.wantClientIP)
			}
		})
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
	if !slices.Contains(corsConfig.ExposeHeaders, "Clear-Site-Data") {
		t.Fatalf("ExposeHeaders = %#v, missing Clear-Site-Data", corsConfig.ExposeHeaders)
	}
	if !slices.Contains(corsConfig.ExposeHeaders, "Location") {
		t.Fatalf("ExposeHeaders = %#v, missing Location", corsConfig.ExposeHeaders)
	}
}

func TestCORSHeaderListsStayMinimalAndDeduplicated(t *testing.T) {
	corsConfig := newCORSConfig(config.Config{
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: []string{"https://app.example.com"},
		},
	})

	wantAllowHeaders := []string{
		"Origin",
		"Content-Type",
		"Accept",
		"Authorization",
		middleware.RequestIDHeader,
	}
	if !slices.Equal(corsConfig.AllowHeaders, wantAllowHeaders) {
		t.Fatalf("AllowHeaders = %#v, want %#v", corsConfig.AllowHeaders, wantAllowHeaders)
	}

	for name, headers := range map[string][]string{
		"AllowHeaders":  corsConfig.AllowHeaders,
		"ExposeHeaders": corsConfig.ExposeHeaders,
	} {
		if duplicates := duplicateHeaderNames(headers); len(duplicates) > 0 {
			t.Fatalf("%s = %#v, duplicate headers = %#v", name, headers, duplicates)
		}
	}

	if hasHeaderName(corsConfig.AllowHeaders, "X-Admin-Override") {
		t.Fatalf("AllowHeaders = %#v, should not allow internal admin override header", corsConfig.AllowHeaders)
	}
	for _, header := range []string{"Set-Cookie", "Cookie", "Authorization"} {
		if hasHeaderName(corsConfig.ExposeHeaders, header) {
			t.Fatalf("ExposeHeaders = %#v, should not expose %s", corsConfig.ExposeHeaders, header)
		}
	}
}

func TestNewCORSConfigKeepsExplicitOriginsAndNoCredentials(t *testing.T) {
	cfg := config.Config{
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: []string{
				" https://app.example.com ",
				"",
				" http://localhost:3000 ",
				"https://app.example.com",
			},
		},
	}

	corsConfig := newCORSConfig(cfg)

	if corsConfig.AllowAllOrigins {
		t.Fatal("AllowAllOrigins must stay disabled")
	}
	if corsConfig.AllowCredentials {
		t.Fatal("AllowCredentials must stay disabled")
	}
	if got := corsConfig.AllowOrigins; !slices.Equal(got, []string{"https://app.example.com", "http://localhost:3000"}) {
		t.Fatalf("AllowOrigins = %#v", got)
	}
}

func TestNormalizedCORSOriginsTrimsEmptyAndDeduplicatesInOrder(t *testing.T) {
	got := normalizedCORSOrigins([]string{
		" https://app.example.com ",
		"",
		"http://localhost:3000",
		"https://app.example.com",
		" http://localhost:3000 ",
	})
	want := []string{"https://app.example.com", "http://localhost:3000"}

	if !slices.Equal(got, want) {
		t.Fatalf("normalizedCORSOrigins() = %#v, want %#v", got, want)
	}
}

func TestNormalizedCORSOriginsCanonicalizesBrowserOriginForm(t *testing.T) {
	got := normalizedCORSOrigins([]string{
		" https://APP.example.com:443 ",
		"https://app.example.com",
		"HTTP://LOCALHOST:80",
		"http://localhost",
		"http://127.0.0.1:3000",
	})
	want := []string{
		"https://app.example.com",
		"http://localhost",
		"http://127.0.0.1:3000",
	}

	if !slices.Equal(got, want) {
		t.Fatalf("normalizedCORSOrigins() = %#v, want %#v", got, want)
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

func TestCORSAllowMethodsStayMinimalAndDeduplicated(t *testing.T) {
	corsConfig := newCORSConfig(config.Config{
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: []string{"https://app.example.com"},
		},
	})

	want := []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodOptions,
	}
	if !slices.Equal(corsConfig.AllowMethods, want) {
		t.Fatalf("AllowMethods = %#v, want %#v", corsConfig.AllowMethods, want)
	}
	if slices.Contains(corsConfig.AllowMethods, http.MethodTrace) || slices.Contains(corsConfig.AllowMethods, http.MethodConnect) {
		t.Fatalf("AllowMethods = %#v, must not expose TRACE or CONNECT", corsConfig.AllowMethods)
	}
	if len(corsConfig.AllowMethods) != len(slices.Compact(slices.Clone(corsConfig.AllowMethods))) {
		t.Fatalf("AllowMethods = %#v, contains duplicates", corsConfig.AllowMethods)
	}
}

func TestCORSAllowsCanonicalizedConfiguredOrigins(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	cfg := config.Config{
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: []string{"https://APP.example.com:443"},
			MaxBodyBytes:     6 * 1024 * 1024,
		},
	}
	router := gin.New()
	applyGlobalMiddleware(router, cfg, middleware.NewHTTPMetrics(prometheus.NewRegistry()))
	router.GET("/api/v1/me", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Origin", "https://app.example.com")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
}

func TestCORSExposeHeadersCoverDocumentedClientHeaders(t *testing.T) {
	corsConfig := newCORSConfig(config.Config{
		HTTP: config.HTTPConfig{
			CORSAllowOrigins: []string{"https://app.example.com"},
		},
	})

	for _, header := range openAPIClientReadableHeaders(t) {
		if !slices.Contains(corsConfig.ExposeHeaders, header) {
			t.Fatalf("ExposeHeaders = %#v, missing documented client header %s", corsConfig.ExposeHeaders, header)
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

func openAPIClientReadableHeaders(t *testing.T) []string {
	t.Helper()

	data := readOpenAPIForServerTest(t)
	var doc serverTestOpenAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	headers := map[string]bool{}
	for _, response := range doc.Components.Responses {
		collectClientHeader(headers, response)
	}
	for _, item := range doc.Paths {
		for _, operation := range item.operations() {
			for _, response := range operation.Responses {
				collectClientHeader(headers, response)
				if resolved := resolveServerTestResponse(response, doc.Components.Responses); resolved != nil {
					collectClientHeader(headers, *resolved)
				}
			}
		}
	}
	return sortedServerTestKeys(headers)
}

func collectClientHeader(headers map[string]bool, response serverTestOpenAPIResponse) {
	for header := range response.Headers {
		if corsExposedByDefault(header) || !clientNeedsCORSExposure(header) {
			continue
		}
		headers[header] = true
	}
}

func corsExposedByDefault(header string) bool {
	switch strings.ToLower(header) {
	case "cache-control", "content-language", "content-length", "content-type", "expires", "last-modified", "pragma":
		return true
	default:
		return false
	}
}

func clientNeedsCORSExposure(header string) bool {
	switch strings.ToLower(header) {
	case "vary":
		return false
	default:
		return true
	}
}

func readOpenAPIForServerTest(t *testing.T) []byte {
	t.Helper()

	candidates := []string{
		filepath.Join("..", "..", "api", "openapi.yaml"),
		filepath.Join("backend", "api", "openapi.yaml"),
	}
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data
		}
	}
	t.Fatalf("read openapi.yaml from %v", candidates)
	return nil
}

type serverTestOpenAPIDocument struct {
	Paths      map[string]serverTestOpenAPIPathItem `yaml:"paths"`
	Components serverTestOpenAPIComponents          `yaml:"components"`
}

type serverTestOpenAPIComponents struct {
	Responses map[string]serverTestOpenAPIResponse `yaml:"responses"`
}

type serverTestOpenAPIPathItem struct {
	// 覆盖 OpenAPI Path Item 的标准 HTTP 操作，避免 CORS 响应头契约漏检。
	Delete  *serverTestOpenAPIOperation `yaml:"delete"`
	Get     *serverTestOpenAPIOperation `yaml:"get"`
	Head    *serverTestOpenAPIOperation `yaml:"head"`
	Options *serverTestOpenAPIOperation `yaml:"options"`
	Patch   *serverTestOpenAPIOperation `yaml:"patch"`
	Post    *serverTestOpenAPIOperation `yaml:"post"`
	Put     *serverTestOpenAPIOperation `yaml:"put"`
	Trace   *serverTestOpenAPIOperation `yaml:"trace"`
}

type serverTestOpenAPIOperation struct {
	Responses map[string]serverTestOpenAPIResponse `yaml:"responses"`
}

type serverTestOpenAPIResponse struct {
	Ref     string         `yaml:"$ref"`
	Headers map[string]any `yaml:"headers"`
}

func (item serverTestOpenAPIPathItem) operations() []*serverTestOpenAPIOperation {
	operations := []*serverTestOpenAPIOperation{
		item.Delete,
		item.Get,
		item.Head,
		item.Options,
		item.Patch,
		item.Post,
		item.Put,
		item.Trace,
	}
	result := make([]*serverTestOpenAPIOperation, 0, len(operations))
	for _, operation := range operations {
		if operation != nil {
			result = append(result, operation)
		}
	}
	return result
}

func resolveServerTestResponse(response serverTestOpenAPIResponse, components map[string]serverTestOpenAPIResponse) *serverTestOpenAPIResponse {
	const prefix = "#/components/responses/"
	if !strings.HasPrefix(response.Ref, prefix) {
		return nil
	}
	if resolved, ok := components[strings.TrimPrefix(response.Ref, prefix)]; ok {
		return &resolved
	}
	return nil
}

func sortedServerTestKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
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
		{" ", ""},
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
			HSTSMaxAgeSeconds:         31536000,
			HSTSIncludeSubDomains:     true,
			HSTSPreload:               true,
			CrossOriginEmbedderPolicy: "require-corp",
		},
	}

	securityConfig := securityHeadersConfig(cfg)

	if securityConfig.HSTSMaxAgeSeconds != 31536000 ||
		!securityConfig.HSTSIncludeSubDomains ||
		!securityConfig.HSTSPreload ||
		securityConfig.CrossOriginEmbedderPolicy != "require-corp" {
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

func TestCORSPreflightAllowsHEADProbes(t *testing.T) {
	router := newCORSMiddlewareTestRouter(t)
	router.HEAD("/readyz", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/readyz", nil)
	req.Header.Set("Origin", "https://app.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodHead)
	req.Header.Set("Access-Control-Request-Headers", middleware.RequestIDHeader)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body = %s", resp.Code, http.StatusNoContent, resp.Body.String())
	}
	if got := resp.Header().Get("Access-Control-Allow-Methods"); !headerHasToken(got, http.MethodHead) {
		t.Fatalf("Access-Control-Allow-Methods = %q, missing %s", got, http.MethodHead)
	}
	if got := resp.Header().Get("Access-Control-Allow-Headers"); !headerHasToken(got, middleware.RequestIDHeader) {
		t.Fatalf("Access-Control-Allow-Headers = %q, missing %s", got, middleware.RequestIDHeader)
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
	for _, header := range []string{"Allow", "Clear-Site-Data", "Content-Disposition", "Link", "Location", "RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset", "WWW-Authenticate", "Retry-After", "X-Total-Count", middleware.RequestIDHeader} {
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

func hasHeaderName(headers []string, name string) bool {
	return slices.ContainsFunc(headers, func(header string) bool {
		return strings.EqualFold(strings.TrimSpace(header), name)
	})
}

func duplicateHeaderNames(headers []string) []string {
	seen := make(map[string]struct{}, len(headers))
	var duplicates []string
	for _, header := range headers {
		normalized := strings.ToLower(strings.TrimSpace(header))
		if _, ok := seen[normalized]; ok {
			duplicates = append(duplicates, header)
			continue
		}
		seen[normalized] = struct{}{}
	}
	return duplicates
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
