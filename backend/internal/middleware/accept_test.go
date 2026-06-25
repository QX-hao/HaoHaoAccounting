package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

func TestAcceptRejectsUnsupportedResponseMediaType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID(), Accept([]AcceptRule{{
		Method:       http.MethodGet,
		Path:         "/api/v1/example",
		OfferedTypes: []string{"application/json"},
	}}))
	router.GET("/api/v1/example", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/example", nil)
	req.Header.Set(RequestIDHeader, "request-123")
	req.Header.Set("Accept", "text/csv")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotAcceptable {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get(RequestIDHeader); got != "request-123" {
		t.Fatalf("request id header = %q", got)
	}
	if got := resp.Header().Get("Vary"); got != "Accept" {
		t.Fatalf("Vary = %q", got)
	}

	var body httputil.ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != httputil.CodeNotAcceptable {
		t.Fatalf("code = %q", body.Code)
	}
	if body.RequestID != "request-123" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
}

func TestAcceptAllowsCompatibleMediaRanges(t *testing.T) {
	for _, header := range []string{
		"",
		"*/*",
		"application/*",
		"application/json",
		"application/json; charset=utf-8",
		"text/csv;q=0, application/json;q=0.5",
	} {
		if !acceptsAnyOfferedType(header, []string{"application/json"}) {
			t.Fatalf("Accept %q should allow application/json", header)
		}
	}
}

func TestAcceptAllowsStructuredSyntaxSuffixRanges(t *testing.T) {
	if !acceptsAnyOfferedType("application/*+json", []string{"application/problem+json"}) {
		t.Fatal("Accept application/*+json should allow application/problem+json")
	}
	if acceptsAnyOfferedType("application/problem+json;q=0, application/*+json;q=1", []string{"application/problem+json"}) {
		t.Fatal("specific application/problem+json;q=0 should override application/*+json")
	}
}

func TestAcceptRejectsExcludedOrIncompatibleMediaRanges(t *testing.T) {
	for _, header := range []string{
		"application/json;q=0",
		"application/json;q=0, */*;q=1",
		"application/*;q=0, */*;q=1",
		"application/*+json",
		"text/csv",
		"application/xml, text/plain",
	} {
		if acceptsAnyOfferedType(header, []string{"application/json"}) {
			t.Fatalf("Accept %q should reject application/json", header)
		}
	}
}

func TestAcceptAddsVaryOnSuccessfulNegotiatedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Accept([]AcceptRule{{
		Method:       http.MethodGet,
		Path:         "/api/v1/example",
		OfferedTypes: []string{"application/json"},
	}}))
	router.GET("/api/v1/example", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/example", nil)
	req.Header.Set("Accept", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get("Vary"); got != "Accept" {
		t.Fatalf("Vary = %q", got)
	}
}

func TestAcceptNormalizesConfiguredOfferedTypesBeforeErrorMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Accept([]AcceptRule{{
		Method:       http.MethodGet,
		Path:         "/api/v1/example",
		OfferedTypes: []string{" Application/JSON ", "application/json", "invalid"},
	}}))
	router.GET("/api/v1/example", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/example", nil)
	req.Header.Set("Accept", "text/csv")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotAcceptable {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if strings.Count(resp.Body.String(), "application/json") != 1 {
		t.Fatalf("body = %s, want one normalized application/json value", resp.Body.String())
	}
	if strings.Contains(resp.Body.String(), "invalid") {
		t.Fatalf("body = %s, leaked invalid offered type", resp.Body.String())
	}
}

func TestAcceptIgnoresRulesWithoutValidMediaTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Accept([]AcceptRule{{
		Method:       http.MethodGet,
		Path:         "/api/v1/example",
		OfferedTypes: []string{"invalid", "application/"},
	}}))
	router.GET("/api/v1/example", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/example", nil)
	req.Header.Set("Accept", "text/csv")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestAcceptFallsBackToStaticOfferedTypesWhenDynamicListIsEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Accept([]AcceptRule{{
		Method:       http.MethodGet,
		Path:         "/api/v1/example",
		OfferedTypes: []string{"application/json"},
		Offered:      func(*gin.Context) []string { return []string{" ", ""} },
	}}))
	router.GET("/api/v1/example", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/example", nil)
	req.Header.Set("Accept", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestAppendVaryPreservesExistingFieldsAndAvoidsDuplicates(t *testing.T) {
	headers := http.Header{}
	headers.Set("Vary", "Origin")

	appendVary(headers, "Accept")
	appendVary(headers, "accept")

	if got := headers.Get("Vary"); got != "Origin, Accept" {
		t.Fatalf("Vary = %q", got)
	}
}

func TestAPIAcceptRulesMatchExportFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Accept(APIAcceptRules()))
	router.GET("/api/v1/io/export", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/io/export?format=csv", nil)
	req.Header.Set("Accept", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotAcceptable {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/io/export?format=xlsx", nil)
	req.Header.Set("Accept", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestAPIAcceptRulesCoverSuccessResponseMediaTypes(t *testing.T) {
	got := acceptRulesByOperation(t, APIAcceptRules())
	want := openAPISuccessResponseMediaTypes(t)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("APIAcceptRules() = %#v, want OpenAPI success response media types %#v", got, want)
	}
}

func acceptRulesByOperation(t *testing.T, rules []AcceptRule) map[string][]string {
	t.Helper()

	result := make(map[string][]string, len(rules))
	for _, rule := range rules {
		method := strings.ToUpper(strings.TrimSpace(rule.Method))
		path := strings.TrimSpace(rule.Path)
		if method == "" || path == "" {
			t.Fatalf("accept rule has empty method or path: %#v", rule)
		}

		offeredTypes := normalizeMediaTypes(rule.OfferedTypes)
		if len(offeredTypes) == 0 {
			t.Fatalf("accept rule has no offered media types: %#v", rule)
		}
		sort.Strings(offeredTypes)

		key := method + " " + path
		if _, exists := result[key]; exists {
			t.Fatalf("duplicate accept rule for %s", key)
		}
		result[key] = offeredTypes
	}
	return result
}

func openAPISuccessResponseMediaTypes(t *testing.T) map[string][]string {
	t.Helper()

	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	result := make(map[string][]string)
	for path, item := range doc.Paths {
		for _, operation := range item.operations() {
			mediaTypes := operation.successResponseMediaTypes(doc.Components.Responses)
			if len(mediaTypes) == 0 {
				continue
			}
			sort.Strings(mediaTypes)
			result[operation.method+" "+openAPIPathToGinPath(path)] = mediaTypes
		}
	}
	return result
}
