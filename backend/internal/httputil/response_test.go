package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

func TestErrorIncludesCodeAndRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(requestIDContextKey, c.GetHeader("X-Request-ID"))
		c.Next()
	})
	router.GET("/boom", func(c *gin.Context) {
		BadRequest(c, "invalid amount")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.Header.Set("X-Request-ID", "request-123")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.Code)
	}

	var body ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error != "invalid amount" {
		t.Fatalf("error = %q", body.Error)
	}
	if body.Code != CodeBadRequest {
		t.Fatalf("code = %q", body.Code)
	}
	if body.RequestID != "request-123" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
}

func TestErrorDoesNotWriteAfterResponseStarted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/already-written", func(c *gin.Context) {
		c.String(http.StatusOK, "already written")
		BadRequest(c, "too late")
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/already-written", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Body.String(); got != "already written" {
		t.Fatalf("body = %q", got)
	}
}

func TestErrorCodesMatchOpenAPIEnum(t *testing.T) {
	got := allErrorCodes()
	want := openAPIErrorCodes(t)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Go error codes = %#v, want OpenAPI enum %#v", got, want)
	}
}

func TestInternalErrorHidesDetailsInReleaseMode(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	InternalError(c, errors.New("database password leaked"))

	var body ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error != "internal server error" {
		t.Fatalf("error = %q", body.Error)
	}
	if body.Code != CodeInternal {
		t.Fatalf("code = %q", body.Code)
	}
}

func TestInternalErrorMapsDeadlineExceededToGatewayTimeout(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	InternalError(c, fmt.Errorf("query transactions: %w", context.DeadlineExceeded))

	var body ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want 504", resp.Code)
	}
	if body.Error != "request timed out" {
		t.Fatalf("error = %q", body.Error)
	}
	if body.Code != CodeRequestTimeout {
		t.Fatalf("code = %q", body.Code)
	}
}

func TestInternalErrorMapsContextCanceledToClientClosedRequest(t *testing.T) {
	previousMode := gin.Mode()
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(previousMode) })

	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	InternalError(c, fmt.Errorf("write response: %w", context.Canceled))

	var body ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp.Code != StatusClientClosedRequest {
		t.Fatalf("status = %d, want %d", resp.Code, StatusClientClosedRequest)
	}
	if body.Error != "client closed request" {
		t.Fatalf("error = %q", body.Error)
	}
	if body.Code != CodeClientClosedRequest {
		t.Fatalf("code = %q", body.Code)
	}
}

func TestUnauthorizedSetsAuthenticateChallenge(t *testing.T) {
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	Unauthorized(c, "invalid token")

	var body ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.Code)
	}
	if got := resp.Header().Get("WWW-Authenticate"); got != `Bearer realm="haohao-accounting-api"` {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
	if body.Code != CodeUnauthorized {
		t.Fatalf("code = %q", body.Code)
	}
}

func TestInvalidTokenSetsBearerErrorChallenge(t *testing.T) {
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	InvalidToken(c, "invalid token")

	var body ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", resp.Code)
	}
	if got := resp.Header().Get("WWW-Authenticate"); got != `Bearer realm="haohao-accounting-api", error="invalid_token"` {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
	if body.Code != CodeUnauthorized {
		t.Fatalf("code = %q", body.Code)
	}
}

func TestBindJSONBodyRejectsUnknownFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body struct {
		Name string `json:"name"`
	}
	c := testContextWithBody(`{"name":"cash","unexpected":true}`)

	if err := BindJSONBody(c, &body); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestBindJSONBodyRejectsMultipleJSONValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body struct {
		Name string `json:"name"`
	}
	c := testContextWithBody(`{"name":"cash"} {"name":"extra"}`)

	if err := BindJSONBody(c, &body); err == nil {
		t.Fatal("expected multiple JSON values error")
	}
}

func TestBindJSONBodyDecodesValidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body struct {
		Name string `json:"name"`
	}
	c := testContextWithBody(`{"name":"cash"}`)

	if err := BindJSONBody(c, &body); err != nil {
		t.Fatalf("bind json body: %v", err)
	}
	if body.Name != "cash" {
		t.Fatalf("name = %q", body.Name)
	}
}

func TestMethodNotAllowedUsesStableCode(t *testing.T) {
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	MethodNotAllowed(c, "method not allowed")

	var body ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", resp.Code)
	}
	if body.Code != CodeMethodNotAllowed {
		t.Fatalf("code = %q", body.Code)
	}
}

func TestNotAcceptableUsesStableCode(t *testing.T) {
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	NotAcceptable(c, "not acceptable")

	var body ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp.Code != http.StatusNotAcceptable {
		t.Fatalf("status = %d", resp.Code)
	}
	if body.Code != CodeNotAcceptable {
		t.Fatalf("code = %q", body.Code)
	}
}

func TestRateLimitedSetsRetryAfter(t *testing.T) {
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	RateLimited(c, "too many requests", 90*time.Second)

	var body ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", resp.Code)
	}
	if got := resp.Header().Get("Retry-After"); got != "90" {
		t.Fatalf("Retry-After = %q", got)
	}
	if body.Code != CodeRateLimited {
		t.Fatalf("code = %q", body.Code)
	}
}

func TestRateLimitedRoundsRetryAfterUp(t *testing.T) {
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	RateLimited(c, "too many requests", 500*time.Millisecond)

	if got := resp.Header().Get("Retry-After"); got != "1" {
		t.Fatalf("Retry-After = %q", got)
	}
}

func TestSetPaginationHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/transactions", func(c *gin.Context) {
		SetPaginationHeaders(c, 45, 2, 20)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/transactions?page=2&pageSize=20&type=expense", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if got := resp.Header().Get("X-Total-Count"); got != "45" {
		t.Fatalf("X-Total-Count = %q", got)
	}
	wantLink := "</transactions?page=1&pageSize=20&type=expense>; rel=\"first\", </transactions?page=1&pageSize=20&type=expense>; rel=\"prev\", </transactions?page=3&pageSize=20&type=expense>; rel=\"next\", </transactions?page=3&pageSize=20&type=expense>; rel=\"last\""
	if got := resp.Header().Get("Link"); got != wantLink {
		t.Fatalf("Link = %q, want %q", got, wantLink)
	}
}

func testContextWithBody(body string) *gin.Context {
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	c.Request = httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestSetPaginationHeadersOmitsLinkForSinglePage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/transactions", func(c *gin.Context) {
		SetPaginationHeaders(c, 20, 1, 20)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/transactions?page=1&pageSize=20", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if got := resp.Header().Get("X-Total-Count"); got != "20" {
		t.Fatalf("X-Total-Count = %q", got)
	}
	if got := resp.Header().Get("Link"); got != "" {
		t.Fatalf("Link = %q, want empty", got)
	}
}

func TestSetPaginationHeadersHandlesLargeTotals(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/transactions", func(c *gin.Context) {
		SetPaginationHeaders(c, math.MaxInt64, 1, 200)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/transactions?page=1&pageSize=200", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if got := resp.Header().Get("X-Total-Count"); got != "9223372036854775807" {
		t.Fatalf("X-Total-Count = %q", got)
	}
	wantLink := "</transactions?page=2&pageSize=200>; rel=\"next\", </transactions?page=46116860184273880&pageSize=200>; rel=\"last\""
	if got := resp.Header().Get("Link"); got != wantLink {
		t.Fatalf("Link = %q, want %q", got, wantLink)
	}
}

func allErrorCodes() []string {
	codes := []string{
		CodeBadRequest,
		CodeInvalidRequest,
		CodeUnauthorized,
		CodeForbidden,
		CodeNotFound,
		CodeMethodNotAllowed,
		CodeRateLimited,
		CodePayloadTooLarge,
		CodeUnsupportedMediaType,
		CodeNotAcceptable,
		CodeRequestTimeout,
		CodeClientClosedRequest,
		CodeInternal,
	}
	sort.Strings(codes)
	return codes
}

func openAPIErrorCodes(t *testing.T) []string {
	t.Helper()

	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	errorResponse, ok := doc.Components.Schemas["ErrorResponse"]
	if !ok {
		t.Fatal("OpenAPI ErrorResponse schema is missing")
	}
	code, ok := errorResponse.Properties["code"]
	if !ok {
		t.Fatal("OpenAPI ErrorResponse.code schema is missing")
	}
	if len(code.Enum) == 0 {
		t.Fatal("OpenAPI ErrorResponse.code enum is empty")
	}
	sort.Strings(code.Enum)
	return code.Enum
}

func readOpenAPI(t *testing.T) []byte {
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

type openAPIDocument struct {
	Components openAPIComponents `yaml:"components"`
}

type openAPIComponents struct {
	Schemas map[string]openAPISchema `yaml:"schemas"`
}

type openAPISchema struct {
	Properties map[string]openAPISchema `yaml:"properties"`
	Enum       []string                 `yaml:"enum"`
}
