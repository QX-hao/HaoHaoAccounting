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
	"strconv"
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
	if body.Status != http.StatusBadRequest {
		t.Fatalf("body status = %d, want 400", body.Status)
	}
	if body.RequestID != "request-123" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
}

func TestErrorOmitsUnsafeRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name  string
		value any
	}{
		{name: "wrong type", value: 42},
		{name: "empty", value: ""},
		{name: "control character", value: "bad\nid"},
		{name: "whitespace", value: "bad id"},
		{name: "too long", value: strings.Repeat("a", 129)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(resp)
			c.Set(requestIDContextKey, tc.value)

			BadRequest(c, "invalid amount")

			var body ErrorResponse
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.RequestID != "" {
				t.Fatalf("requestId = %q, want empty", body.RequestID)
			}
		})
	}
}

func TestErrorIncludesStableEnvelopeFields(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)

	BadRequest(c, "invalid amount")

	var body map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	value, ok := body["requestId"]
	if !ok {
		t.Fatalf("requestId missing from %#v", body)
	}
	if value != "" {
		t.Fatalf("requestId = %#v, want empty string", value)
	}
	if value, ok := body["status"]; !ok {
		t.Fatalf("status missing from %#v", body)
	} else if value != float64(http.StatusBadRequest) {
		t.Fatalf("status = %#v, want 400", value)
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

func TestErrorRecordsNonSensitiveLogSummary(t *testing.T) {
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)

	Error(c, http.StatusBadRequest, CodeBadRequest, "raw user input: password=secret")

	if got := c.Errors.ByType(gin.ErrorTypePrivate).String(); got != "Error #01: status=400 code=bad_request\n" {
		t.Fatalf("gin private error = %q", got)
	}
	got := c.Errors.ByType(gin.ErrorTypePrivate).String()
	if strings.Contains(got, "password=secret") || strings.Contains(got, "raw user input") {
		t.Fatalf("gin private error leaked response message: %q", got)
	}
}

func TestErrorRejectsNonErrorStatuses(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusFound, 700} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			resp := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(resp)

			Error(c, status, CodeBadRequest, "caller supplied non-error status")

			if resp.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", resp.Code)
			}
			var body ErrorResponse
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body.Status != http.StatusInternalServerError {
				t.Fatalf("body status = %d, want 500", body.Status)
			}
			if body.Code != CodeInternal {
				t.Fatalf("code = %q, want %q", body.Code, CodeInternal)
			}
			if body.Error != "internal server error" {
				t.Fatalf("error = %q", body.Error)
			}
			if strings.Contains(resp.Body.String(), "caller supplied") {
				t.Fatalf("response leaked caller message: %s", resp.Body.String())
			}
		})
	}
}

func TestErrorDefaultsStableCodesForKnownStatuses(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   string
	}{
		{status: http.StatusBadRequest, code: CodeBadRequest},
		{status: http.StatusUnauthorized, code: CodeUnauthorized},
		{status: http.StatusForbidden, code: CodeForbidden},
		{status: http.StatusNotFound, code: CodeNotFound},
		{status: http.StatusMethodNotAllowed, code: CodeMethodNotAllowed},
		{status: http.StatusTooManyRequests, code: CodeRateLimited},
		{status: http.StatusRequestEntityTooLarge, code: CodePayloadTooLarge},
		{status: http.StatusUnsupportedMediaType, code: CodeUnsupportedMediaType},
		{status: http.StatusNotAcceptable, code: CodeNotAcceptable},
		{status: http.StatusGatewayTimeout, code: CodeRequestTimeout},
		{status: StatusClientClosedRequest, code: CodeClientClosedRequest},
		{status: http.StatusInternalServerError, code: CodeInternal},
		{status: http.StatusBadGateway, code: CodeInternal},
	} {
		t.Run(strconv.Itoa(tc.status), func(t *testing.T) {
			resp := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(resp)

			Error(c, tc.status, "", "request failed")

			var body ErrorResponse
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if resp.Code != tc.status {
				t.Fatalf("status = %d, want %d", resp.Code, tc.status)
			}
			if body.Status != tc.status {
				t.Fatalf("body status = %d, want %d", body.Status, tc.status)
			}
			if body.Code != tc.code {
				t.Fatalf("code = %q, want %q", body.Code, tc.code)
			}
		})
	}
}

func TestErrorHelpersUseDocumentedStatusAndCode(t *testing.T) {
	for _, tc := range []struct {
		name   string
		write  func(*gin.Context)
		status int
		code   string
	}{
		{name: "BadRequest", write: func(c *gin.Context) { BadRequest(c, "bad request") }, status: http.StatusBadRequest, code: CodeBadRequest},
		{name: "InvalidRequest", write: func(c *gin.Context) { InvalidRequest(c, "invalid request") }, status: http.StatusBadRequest, code: CodeInvalidRequest},
		{name: "Unauthorized", write: func(c *gin.Context) { Unauthorized(c, "unauthorized") }, status: http.StatusUnauthorized, code: CodeUnauthorized},
		{name: "InvalidToken", write: func(c *gin.Context) { InvalidToken(c, "invalid token") }, status: http.StatusUnauthorized, code: CodeUnauthorized},
		{name: "Forbidden", write: func(c *gin.Context) { Forbidden(c, "forbidden") }, status: http.StatusForbidden, code: CodeForbidden},
		{name: "NotFound", write: func(c *gin.Context) { NotFound(c, "not found") }, status: http.StatusNotFound, code: CodeNotFound},
		{name: "MethodNotAllowed", write: func(c *gin.Context) { MethodNotAllowed(c, "method not allowed") }, status: http.StatusMethodNotAllowed, code: CodeMethodNotAllowed},
		{name: "RateLimited", write: func(c *gin.Context) { RateLimited(c, "too many requests", 0) }, status: http.StatusTooManyRequests, code: CodeRateLimited},
		{name: "RateLimitedWithPolicy", write: func(c *gin.Context) { RateLimitedWithPolicy(c, "too many requests", 0, 5, 0) }, status: http.StatusTooManyRequests, code: CodeRateLimited},
		{name: "PayloadTooLarge", write: func(c *gin.Context) { PayloadTooLarge(c, "payload too large") }, status: http.StatusRequestEntityTooLarge, code: CodePayloadTooLarge},
		{name: "UnsupportedMediaType", write: func(c *gin.Context) { UnsupportedMediaType(c, "unsupported media type") }, status: http.StatusUnsupportedMediaType, code: CodeUnsupportedMediaType},
		{name: "NotAcceptable", write: func(c *gin.Context) { NotAcceptable(c, "not acceptable") }, status: http.StatusNotAcceptable, code: CodeNotAcceptable},
		{name: "GatewayTimeout", write: func(c *gin.Context) { GatewayTimeout(c, "request timed out") }, status: http.StatusGatewayTimeout, code: CodeRequestTimeout},
		{name: "ClientClosedRequest", write: func(c *gin.Context) { ClientClosedRequest(c, "client closed request") }, status: StatusClientClosedRequest, code: CodeClientClosedRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(resp)

			tc.write(c)

			var body ErrorResponse
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if resp.Code != tc.status {
				t.Fatalf("status = %d, want %d", resp.Code, tc.status)
			}
			if body.Status != tc.status {
				t.Fatalf("body status = %d, want %d", body.Status, tc.status)
			}
			if body.Code != tc.code {
				t.Fatalf("code = %q, want %q", body.Code, tc.code)
			}
		})
	}
}

func TestReadmeDocumentsHTTPUtilityContracts(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"`Error`",
		"`error`, `code`, `status`, and `requestId`",
		"non-error HTTP statuses",
		"`500 internal_error`",
		"Gin private error summary",
		"already-started response",
		"`InternalError`",
		"`context.DeadlineExceeded`",
		"`context.Canceled`",
		"non-standard `499`",
		"`client_closed_request`",
		"`WWW-Authenticate` bearer challenges",
		"`RateLimitedWithPolicy`",
		"`Retry-After`",
		"`RateLimit-Limit`",
		"`RateLimit-Remaining`",
		"`RateLimit-Reset`",
		"non-negative integer seconds",
		"`BindJSONBody`",
		"`DisallowUnknownFields`",
		"multiple JSON values",
		"Gin `binding` tag validation",
		"documented request schema",
		"`BindQuery`",
		"query binding",
		"`invalid_request` response",
		"query parameters",
		"`X-Total-Count`",
		"RFC 8288 `Link` headers",
		"`SetCreatedLocation`",
		"relative `Location` header",
		"created or queued resources",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("README.md is missing HTTP utility guidance %q", want)
		}
	}
}

func TestHandlersUseStrictJSONBindingHelper(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("..", "modules", "*", "*.go"))
	if err != nil {
		t.Fatalf("glob module files: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("no module Go files found")
	}

	checkedHandlers := 0
	for _, path := range matches {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		source := string(data)
		if !strings.HasSuffix(path, "handler.go") {
			if strings.Contains(source, "BindJSONBody(") || strings.Contains(source, "ShouldBindJSON(") {
				t.Fatalf("%s contains JSON binding outside handler.go", path)
			}
			continue
		}
		checkedHandlers++
		// handler 必须走 BindJSONBody，保持服务端严格 JSON 解析和 OpenAPI closed schema 一致。
		if strings.Contains(source, "ShouldBindJSON(") {
			t.Fatalf("%s uses ShouldBindJSON directly; use BindJSONBody", path)
		}
		if strings.Contains(source, "BindJSONBody(") && !strings.Contains(source, "HandleBodyReadError(c, err)") {
			t.Fatalf("%s uses BindJSONBody without first mapping MaxBytesReader errors through HandleBodyReadError", path)
		}
	}
	if checkedHandlers == 0 {
		t.Fatal("no module handler.go files found")
	}
}

func TestErrorCodesMatchOpenAPIEnum(t *testing.T) {
	got := allErrorCodes()
	want := openAPIErrorCodes(t)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Go error codes = %#v, want OpenAPI enum %#v", got, want)
	}
}

func TestErrorResponseRequiredFieldsMatchOpenAPI(t *testing.T) {
	got := errorResponseJSONFields(t)
	want := openAPIErrorResponseRequiredFields(t)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Go ErrorResponse fields = %#v, want OpenAPI required fields %#v", got, want)
	}
}

func TestClientClosedRequestMatchesOpenAPIResponseComponent(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	response, ok := doc.Components.Responses["ClientClosedRequest"]
	if !ok {
		t.Fatal("OpenAPI components.responses.ClientClosedRequest is missing")
	}
	media, ok := response.Content["application/json"]
	if !ok {
		t.Fatal("ClientClosedRequest response is missing application/json content")
	}
	if media.Schema.Ref != "#/components/schemas/ErrorResponse" {
		t.Fatalf("ClientClosedRequest schema ref = %q", media.Schema.Ref)
	}
	if media.Example.Status != StatusClientClosedRequest {
		t.Fatalf("ClientClosedRequest example status = %d, want %d", media.Example.Status, StatusClientClosedRequest)
	}
	if media.Example.Code != CodeClientClosedRequest {
		t.Fatalf("ClientClosedRequest example code = %q, want %q", media.Example.Code, CodeClientClosedRequest)
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
	if got := resp.Header().Get("WWW-Authenticate"); got != `Bearer realm="haohao-accounting-api", error="invalid_token", error_description="The access token is missing, expired, revoked, or invalid"` {
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

func TestBindJSONBodyRejectsEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body struct {
		Name string `json:"name"`
	}
	c := testContextWithBody("")

	if err := BindJSONBody(c, &body); err == nil {
		t.Fatal("expected empty JSON body error")
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

func TestBindJSONBodyValidatesBindingTags(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body struct {
		Name string `json:"name" binding:"required,min=1"`
	}
	c := testContextWithBody(`{"name":""}`)

	if err := BindJSONBody(c, &body); err == nil {
		t.Fatal("expected binding validation error")
	}
}

func TestBindQueryUsesGinQueryBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var query struct {
		Page int `form:"page" binding:"min=1"`
	}
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	c.Request = httptest.NewRequest(http.MethodGet, "/items?page=2", nil)

	if err := BindQuery(c, &query); err != nil {
		t.Fatalf("bind query: %v", err)
	}
	if query.Page != 2 {
		t.Fatalf("page = %d, want 2", query.Page)
	}
}

func TestBindQueryReturnsBindingErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var query struct {
		Page int `form:"page" binding:"min=1"`
	}
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	c.Request = httptest.NewRequest(http.MethodGet, "/items?page=0", nil)

	if err := BindQuery(c, &query); err == nil {
		t.Fatal("expected query binding validation error")
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

func TestRateLimitedOmitsRetryAfterWhenDelayIsNotPositive(t *testing.T) {
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	RateLimited(c, "too many requests", 0)

	if got := resp.Header().Get("Retry-After"); got != "" {
		t.Fatalf("Retry-After = %q, want empty", got)
	}
}

func TestRateLimitedWithPolicySetsStandardRateLimitHeaders(t *testing.T) {
	resp := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(resp)
	RateLimitedWithPolicy(c, "too many requests", 1500*time.Millisecond, 5, 0)

	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", resp.Code)
	}
	for key, want := range map[string]string{
		"Retry-After":         "2",
		"RateLimit-Limit":     "5",
		"RateLimit-Remaining": "0",
		"RateLimit-Reset":     "2",
	} {
		if got := resp.Header().Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
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

func TestSetPaginationHeadersDropsSensitiveQueryParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/transactions", func(c *gin.Context) {
		SetPaginationHeaders(c, 45, 2, 20)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/transactions?page=2&pageSize=20&type=expense&access_token=secret&token=secret&password=secret", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	link := resp.Header().Get("Link")
	for _, leaked := range []string{"access_token=", "token=", "password=", "secret"} {
		if strings.Contains(link, leaked) {
			t.Fatalf("Link header leaked sensitive query parameter %q: %s", leaked, link)
		}
	}
	if !strings.Contains(link, "type=expense") {
		t.Fatalf("Link header dropped non-sensitive query parameter: %s", link)
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

func TestSetPaginationHeadersSkipsInvalidPaginationInputs(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/transactions", func(c *gin.Context) {
		SetPaginationHeaders(c, -1, 0, 0)
		c.Status(http.StatusNoContent)
	})

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/transactions?page=0&pageSize=0", nil)
	router.ServeHTTP(resp, req)

	if got := resp.Header().Get("X-Total-Count"); got != "" {
		t.Fatalf("X-Total-Count = %q, want empty", got)
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

func TestSetCreatedLocation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/accounts", func(c *gin.Context) {
		SetCreatedLocation(c, 42)
		c.Status(http.StatusCreated)
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/accounts", nil))

	if got := resp.Header().Get("Location"); got != "/accounts/42" {
		t.Fatalf("Location = %q, want /accounts/42", got)
	}
}

func TestSetResourceLocationForAcceptedJob(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/io/import/jobs", func(c *gin.Context) {
		SetResourceLocation(c, 42)
		c.Status(http.StatusAccepted)
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/io/import/jobs", nil))

	if got := resp.Header().Get("Location"); got != "/io/import/jobs/42" {
		t.Fatalf("Location = %q, want /io/import/jobs/42", got)
	}
}

func TestSetCreatedLocationIgnoresZeroID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.POST("/accounts", func(c *gin.Context) {
		SetCreatedLocation(c, 0)
		c.Status(http.StatusCreated)
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/accounts", nil))

	if got := resp.Header().Get("Location"); got != "" {
		t.Fatalf("Location = %q, want empty", got)
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

func errorResponseJSONFields(t *testing.T) []string {
	t.Helper()

	responseType := reflect.TypeOf(ErrorResponse{})
	fields := make([]string, 0, responseType.NumField())
	for i := 0; i < responseType.NumField(); i++ {
		field := responseType.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || name == "-" {
			continue
		}
		fields = append(fields, name)
	}
	sort.Strings(fields)
	return fields
}

func openAPIErrorResponseRequiredFields(t *testing.T) []string {
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
	required := append([]string(nil), errorResponse.Required...)
	sort.Strings(required)
	return required
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
	Responses map[string]openAPIResponse `yaml:"responses"`
	Schemas   map[string]openAPISchema   `yaml:"schemas"`
}

type openAPISchema struct {
	Properties map[string]openAPISchema `yaml:"properties"`
	Ref        string                   `yaml:"$ref"`
	Enum       []string                 `yaml:"enum"`
	Required   []string                 `yaml:"required"`
}

type openAPIResponse struct {
	Content map[string]openAPIMediaType `yaml:"content"`
}

type openAPIMediaType struct {
	Schema  openAPISchema `yaml:"schema"`
	Example ErrorResponse `yaml:"example"`
}
