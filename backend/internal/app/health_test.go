package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/config"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"github.com/gin-gonic/gin"
)

type healthBody struct {
	Status string                       `json:"status"`
	Checks map[string]map[string]string `json:"checks"`
}

type errorBody struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"requestId,omitempty"`
}

type pingFunc func(context.Context) error

func (fn pingFunc) Ping(ctx context.Context) error {
	return fn(ctx)
}

func TestHealthLivezReturnsOK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerHealthRoutes(router, nil, nil)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/livez", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"status":"ok"`) {
		t.Fatalf("body = %s", resp.Body.String())
	}
	assertNoCacheHeaders(t, resp)
}

func TestHealthProbeHEADReturnsStatusAndHeadersOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerHealthRoutes(router, nil, nil)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodHead, "/livez", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if resp.Body.Len() != 0 {
		t.Fatalf("HEAD body = %q, want empty", resp.Body.String())
	}
	assertNoCacheHeaders(t, resp)
}

func TestReadmeDocumentsRouteContracts(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"`/livez` only reports process liveness",
		"`/readyz` and `/health` check the database and optional Redis cache",
		"`/metrics` owned by the server entrypoint",
		"2 second dependency budget",
		"Database or Redis failures return `503` with `status: unavailable`",
		"do not expose raw dependency error details",
		"Redis is reported as `disabled`",
		"All `/api/v1` routes use `NoStore` cache headers",
		"API fallback errors for missing routes and unsupported methods",
		"shared structured error body with request IDs",
		"`Cache-Control: no-store`",
		"`Pragma: no-cache`",
		"`Expires: 0`",
		"Health probe responses use `Cache-Control: no-cache`",
		"Health probes support both `GET` and `HEAD`",
		"Non-API health probe fallbacks remain cache-neutral",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("README.md is missing app route guidance %q", want)
		}
	}
}

func TestAPIRoutesDisableCachingAndHealthProbesRequireRevalidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	if err := RegisterRoutesWithConfig(router, testutil.NewStore(t), nil, config.Config{
		Admin: config.AdminConfig{
			Username: "admin",
			Password: "secret-password",
			Name:     "管理员",
		},
		LoginRateLimit: config.LoginRateLimitConfig{MaxFailures: 5, Window: time.Minute},
		JWT:            config.JWTConfig{Secret: "test-jwt-secret-with-at-least-32-chars", TTL: time.Hour, Issuer: "issuer", Audience: "api"},
	}); err != nil {
		t.Fatalf("register routes: %v", err)
	}

	apiResp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"secret-password"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(apiResp, req)

	if apiResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", apiResp.Code, apiResp.Body.String())
	}
	if got := apiResp.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("api Cache-Control = %q", got)
	}
	if got := apiResp.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("api Pragma = %q", got)
	}
	if got := apiResp.Header().Get("Expires"); got != "0" {
		t.Fatalf("api Expires = %q", got)
	}

	healthResp := httptest.NewRecorder()
	router.ServeHTTP(healthResp, httptest.NewRequest(http.MethodGet, "/livez", nil))

	if healthResp.Code != http.StatusOK {
		t.Fatalf("livez status = %d, body = %s", healthResp.Code, healthResp.Body.String())
	}
	assertNoCacheHeaders(t, healthResp)
}

func TestFallbackRoutesReturnStructuredErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.RequestID())
	if err := RegisterRoutesWithConfig(router, testutil.NewStore(t), nil, config.Config{
		Admin: config.AdminConfig{
			Username: "admin",
			Password: "secret-password",
			Name:     "管理员",
		},
		LoginRateLimit: config.LoginRateLimitConfig{MaxFailures: 5, Window: time.Minute},
		JWT:            config.JWTConfig{Secret: "test-jwt-secret-with-at-least-32-chars", TTL: time.Hour, Issuer: "issuer", Audience: "api"},
	}); err != nil {
		t.Fatalf("register routes: %v", err)
	}

	notFoundReq := httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
	notFoundReq.Header.Set(middleware.RequestIDHeader, "request-404")
	notFoundResp := httptest.NewRecorder()
	router.ServeHTTP(notFoundResp, notFoundReq)

	if notFoundResp.Code != http.StatusNotFound {
		t.Fatalf("not found status = %d, body = %s", notFoundResp.Code, notFoundResp.Body.String())
	}
	if got := notFoundResp.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("api not found Cache-Control = %q", got)
	}
	if got := notFoundResp.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("api not found Pragma = %q", got)
	}
	if got := notFoundResp.Header().Get("Expires"); got != "0" {
		t.Fatalf("api not found Expires = %q", got)
	}
	notFoundBody := parseErrorBody(t, notFoundResp)
	if notFoundBody.Code != httputil.CodeNotFound || notFoundBody.RequestID != "request-404" {
		t.Fatalf("not found body = %#v", notFoundBody)
	}

	methodReq := httptest.NewRequest(http.MethodPatch, "/livez", nil)
	methodReq.Header.Set(middleware.RequestIDHeader, "request-405")
	methodResp := httptest.NewRecorder()
	router.ServeHTTP(methodResp, methodReq)

	if methodResp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("method status = %d, body = %s", methodResp.Code, methodResp.Body.String())
	}
	if got := methodResp.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("Allow = %q", got)
	}
	if got := methodResp.Header().Get("Cache-Control"); got != "" {
		t.Fatalf("livez method Cache-Control = %q, want empty", got)
	}
	methodBody := parseErrorBody(t, methodResp)
	if methodBody.Code != httputil.CodeMethodNotAllowed || methodBody.RequestID != "request-405" {
		t.Fatalf("method body = %#v", methodBody)
	}

	apiMethodReq := httptest.NewRequest(http.MethodPatch, "/api/v1/accounts", nil)
	apiMethodReq.Header.Set(middleware.RequestIDHeader, "request-api-405")
	apiMethodResp := httptest.NewRecorder()
	router.ServeHTTP(apiMethodResp, apiMethodReq)

	if apiMethodResp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("api method status = %d, body = %s", apiMethodResp.Code, apiMethodResp.Body.String())
	}
	if got := apiMethodResp.Header().Get("Allow"); got != "GET, POST" {
		t.Fatalf("api Allow = %q", got)
	}
	if got := apiMethodResp.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("api method Cache-Control = %q", got)
	}
	apiMethodBody := parseErrorBody(t, apiMethodResp)
	if apiMethodBody.Code != httputil.CodeMethodNotAllowed || apiMethodBody.RequestID != "request-api-405" {
		t.Fatalf("api method body = %#v", apiMethodBody)
	}
}

func TestNormalizeAllowHeaderUsesStableMethodOrder(t *testing.T) {
	headers := http.Header{}
	headers.Set("Allow", "POST, get, GET, DELETE, PATCH, UNKNOWN, OPTIONS, HEAD, PUT")

	normalizeAllowHeader(headers)

	if got := headers.Get("Allow"); got != "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS, UNKNOWN" {
		t.Fatalf("Allow = %q", got)
	}
}

func TestLoginRateLimitContractThroughRegisteredRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(middleware.RequestID())
	if err := RegisterRoutesWithConfig(router, testutil.NewStore(t), nil, config.Config{
		Admin: config.AdminConfig{
			Username: "admin",
			Password: "secret-password",
			Name:     "管理员",
		},
		LoginRateLimit: config.LoginRateLimitConfig{MaxFailures: 1, Window: time.Minute},
		JWT:            config.JWTConfig{Secret: "test-jwt-secret-with-at-least-32-chars", TTL: time.Hour, ClockSkew: 30 * time.Second, Issuer: "issuer", Audience: "api"},
	}); err != nil {
		t.Fatalf("register routes: %v", err)
	}

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(middleware.RequestIDHeader, "request-limit")
		req.RemoteAddr = "192.0.2.44:1234"
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)

		if i == 0 {
			if resp.Code != http.StatusUnauthorized {
				t.Fatalf("first status = %d, body = %s", resp.Code, resp.Body.String())
			}
			continue
		}

		if resp.Code != http.StatusTooManyRequests {
			t.Fatalf("blocked status = %d, body = %s", resp.Code, resp.Body.String())
		}
		if got := resp.Header().Get("Retry-After"); got != "60" {
			t.Fatalf("Retry-After = %q", got)
		}
		if got := resp.Header().Get("RateLimit-Limit"); got != "1" {
			t.Fatalf("RateLimit-Limit = %q", got)
		}
		if got := resp.Header().Get("RateLimit-Remaining"); got != "0" {
			t.Fatalf("RateLimit-Remaining = %q", got)
		}
		if got := resp.Header().Get("RateLimit-Reset"); got != "60" {
			t.Fatalf("RateLimit-Reset = %q", got)
		}
		if got := resp.Header().Get(middleware.RequestIDHeader); got != "request-limit" {
			t.Fatalf("request id header = %q", got)
		}
		if got := resp.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("Cache-Control = %q", got)
		}
		body := parseErrorBody(t, resp)
		if body.Code != httputil.CodeRateLimited || body.RequestID != "request-limit" {
			t.Fatalf("body = %#v", body)
		}
	}
}

func TestReadyzReturnsOKWhenDatabaseIsReadyAndRedisDisabled(t *testing.T) {
	router := gin.New()
	router.GET("/readyz", readyzWithDependencies(pingFunc(func(context.Context) error {
		return nil
	}), nil))

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := parseHealthBody(t, resp)
	if body.Status != "ok" || body.Checks["database"]["status"] != "ok" || body.Checks["redis"]["status"] != "disabled" {
		t.Fatalf("body = %#v", body)
	}
	assertNoCacheHeaders(t, resp)
}

func TestReadyzReturnsUnavailableWhenDatabaseFails(t *testing.T) {
	const internalError = "database down host=db.internal password=secret"

	router := gin.New()
	router.GET("/readyz", readyzWithDependencies(pingFunc(func(context.Context) error {
		return errors.New(internalError)
	}), nil))

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := parseHealthBody(t, resp)
	if body.Status != "unavailable" || body.Checks["database"]["status"] != "error" || body.Checks["database"]["error"] != "unavailable" {
		t.Fatalf("body = %#v", body)
	}
	if strings.Contains(resp.Body.String(), internalError) || strings.Contains(resp.Body.String(), "secret") {
		t.Fatalf("health response leaked internal dependency error: %s", resp.Body.String())
	}
	assertNoCacheHeaders(t, resp)
}

func TestReadyzHEADPreservesDependencyStatus(t *testing.T) {
	router := gin.New()
	router.HEAD("/readyz", readyzWithDependencies(pingFunc(func(context.Context) error {
		return errors.New("database down")
	}), nil))

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodHead, "/readyz", nil))

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.Code)
	}
	if resp.Body.Len() != 0 {
		t.Fatalf("HEAD body = %q, want empty", resp.Body.String())
	}
	assertNoCacheHeaders(t, resp)
}

func TestReadyzReturnsUnavailableWhenRedisFails(t *testing.T) {
	const internalError = "redis down addr=redis.internal password=secret"

	router := gin.New()
	router.GET("/readyz", readyzWithDependencies(
		pingFunc(func(context.Context) error { return nil }),
		pingFunc(func(context.Context) error { return errors.New(internalError) }),
	))

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := parseHealthBody(t, resp)
	if body.Status != "unavailable" || body.Checks["redis"]["status"] != "error" || body.Checks["redis"]["error"] != "unavailable" {
		t.Fatalf("body = %#v", body)
	}
	if strings.Contains(resp.Body.String(), internalError) || strings.Contains(resp.Body.String(), "secret") {
		t.Fatalf("health response leaked internal dependency error: %s", resp.Body.String())
	}
	assertNoCacheHeaders(t, resp)
}

func assertNoCacheHeaders(t *testing.T, resp *httptest.ResponseRecorder) {
	t.Helper()

	if got := resp.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := resp.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma = %q", got)
	}
	if got := resp.Header().Get("Expires"); got != "0" {
		t.Fatalf("Expires = %q", got)
	}
}

func parseErrorBody(t *testing.T, resp *httptest.ResponseRecorder) errorBody {
	t.Helper()
	var body errorBody
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	return body
}

func parseHealthBody(t *testing.T, resp *httptest.ResponseRecorder) healthBody {
	t.Helper()
	var body healthBody
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode health body: %v", err)
	}
	return body
}
