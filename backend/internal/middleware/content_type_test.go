package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

func TestContentTypeRejectsUnsupportedMediaType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID(), ContentType([]ContentTypeRule{{
		Method:       http.MethodPost,
		Path:         "/api/v1/example",
		AllowedTypes: []string{"application/json"},
	}}))
	router.POST("/api/v1/example", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/example", strings.NewReader("{}"))
	req.Header.Set(RequestIDHeader, "request-123")
	req.Header.Set("Content-Type", "text/plain")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get(RequestIDHeader); got != "request-123" {
		t.Fatalf("request id header = %q", got)
	}

	var body httputil.ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != httputil.CodeUnsupportedMediaType {
		t.Fatalf("code = %q", body.Code)
	}
	if body.RequestID != "request-123" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
}

func TestContentTypeAllowsParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(ContentType([]ContentTypeRule{{
		Method:       http.MethodPost,
		Path:         "/upload",
		AllowedTypes: []string{"multipart/form-data"},
	}}))
	router.POST("/upload", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(""))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=abc")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestContentTypeAllowsStructuredJSONMediaTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(ContentType([]ContentTypeRule{{
		Method:       http.MethodPost,
		Path:         "/json",
		AllowedTypes: []string{"application/json"},
	}}))
	router.POST("/json", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/vnd.haohao+json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestContentTypeRejectsStructuredJSONOutsideApplicationType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(ContentType([]ContentTypeRule{{
		Method:       http.MethodPost,
		Path:         "/json",
		AllowedTypes: []string{"application/json"},
	}}))
	router.POST("/json", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/problem+json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415, body = %s", resp.Code, resp.Body.String())
	}
}

func TestContentTypeRejectsMalformedHeaderParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handlerCalled := false
	router := gin.New()
	router.Use(ContentType([]ContentTypeRule{{
		Method:       http.MethodPost,
		Path:         "/json",
		AllowedTypes: []string{"application/json"},
	}}))
	router.POST("/json", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader("{}"))
	req.Header.Set("Content-Type", `application/json; charset="unterminated`)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want 415, body = %s", resp.Code, resp.Body.String())
	}
	if handlerCalled {
		t.Fatal("handler ran after malformed Content-Type header")
	}
}

func TestContentTypeIgnoresUnmatchedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(ContentType([]ContentTypeRule{{
		Method:       http.MethodPost,
		Path:         "/api/v1/example",
		AllowedTypes: []string{"application/json"},
	}}))
	router.GET("/api/v1/example", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/example", nil))

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestContentTypeIgnoresRulesWithoutValidMediaTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(ContentType([]ContentTypeRule{{
		Method:       http.MethodPost,
		Path:         "/api/v1/example",
		AllowedTypes: []string{"invalid", "application/"},
	}}))
	router.POST("/api/v1/example", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/example", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestContentTypeIgnoresWildcardAllowedTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(ContentType([]ContentTypeRule{{
		Method:       http.MethodPost,
		Path:         "/api/v1/example",
		AllowedTypes: []string{"*/*", "application/*", "application/*+json"},
	}}))
	router.POST("/api/v1/example", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/example", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "text/plain")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestNormalizeMediaTypesDeduplicatesAndRejectsInvalidValues(t *testing.T) {
	got := normalizeMediaTypes([]string{
		" Application/JSON ",
		"application/json",
		"text/csv",
		"*/*",
		"application/*",
		"application/*+json",
		"application/json; charset=utf-8",
		"text/csv; header=present",
		"application/json ; charset=utf-8",
		"invalid",
		"",
		"application/",
	})
	want := []string{"application/json", "text/csv"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeMediaTypes() = %#v, want %#v", got, want)
	}
}

func TestAPIMediaTypeRulesCoverRequestBodyOperations(t *testing.T) {
	got := mediaTypeRulesByOperation(t, APIMediaTypeRules())
	want := openAPIRequestBodyMediaTypes(t)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("APIMediaTypeRules() = %#v, want OpenAPI request bodies %#v", got, want)
	}
}

func mediaTypeRulesByOperation(t *testing.T, rules []ContentTypeRule) map[string][]string {
	t.Helper()

	result := make(map[string][]string, len(rules))
	for _, rule := range rules {
		method := strings.ToUpper(strings.TrimSpace(rule.Method))
		path := strings.TrimSpace(rule.Path)
		if method == "" || path == "" {
			t.Fatalf("content type rule has empty method or path: %#v", rule)
		}

		allowedTypes := normalizeMediaTypes(rule.AllowedTypes)
		if len(allowedTypes) == 0 {
			t.Fatalf("content type rule has no allowed media types: %#v", rule)
		}
		sort.Strings(allowedTypes)

		key := method + " " + path
		if _, exists := result[key]; exists {
			t.Fatalf("duplicate content type rule for %s", key)
		}
		result[key] = allowedTypes
	}
	return result
}

func openAPIRequestBodyMediaTypes(t *testing.T) map[string][]string {
	t.Helper()

	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	result := make(map[string][]string)
	for path, item := range doc.Paths {
		for _, operation := range item.operations() {
			if operation.requestBody == nil {
				continue
			}
			if len(operation.requestBody.Content) == 0 {
				t.Fatalf("OpenAPI operation %s %s has requestBody without content", operation.method, path)
			}

			mediaTypes := make([]string, 0, len(operation.requestBody.Content))
			for mediaType := range operation.requestBody.Content {
				mediaType = strings.ToLower(strings.TrimSpace(mediaType))
				if mediaType != "" {
					mediaTypes = append(mediaTypes, mediaType)
				}
			}
			sort.Strings(mediaTypes)
			result[operation.method+" "+openAPIPathToGinPath(path)] = mediaTypes
		}
	}
	return result
}

func readOpenAPI(t *testing.T) []byte {
	t.Helper()

	candidates := []string{
		filepath.Join("..", "..", "api", "openapi.yaml"),
		filepath.Join("backend", "api", "openapi.yaml"),
	}

	var lastErr error
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data
		}
		lastErr = err
	}
	t.Fatalf("read openapi.yaml from %v: %v", candidates, lastErr)
	return nil
}

type openAPIDocument struct {
	Paths      map[string]openAPIPathItem `yaml:"paths"`
	Components openAPIComponents          `yaml:"components"`
}

type openAPIComponents struct {
	Responses map[string]openAPIResponse `yaml:"responses"`
}

type openAPIPathItem struct {
	// 覆盖 OpenAPI Path Item 的标准 HTTP 操作，避免契约测试漏掉未来新增的方法。
	Delete  *openAPIOperation `yaml:"delete"`
	Get     *openAPIOperation `yaml:"get"`
	Head    *openAPIOperation `yaml:"head"`
	Options *openAPIOperation `yaml:"options"`
	Patch   *openAPIOperation `yaml:"patch"`
	Post    *openAPIOperation `yaml:"post"`
	Put     *openAPIOperation `yaml:"put"`
	Trace   *openAPIOperation `yaml:"trace"`
}

type openAPIOperation struct {
	RequestBody *openAPIRequestBody        `yaml:"requestBody"`
	Responses   map[string]openAPIResponse `yaml:"responses"`
}

type openAPIRequestBody struct {
	Content map[string]any `yaml:"content"`
}

type openAPIResponse struct {
	Ref     string         `yaml:"$ref"`
	Content map[string]any `yaml:"content"`
}

type openAPIOperationWithMethod struct {
	method       string
	requestBody  *openAPIRequestBody
	responseByID map[string]openAPIResponse
}

func (item openAPIPathItem) operations() []openAPIOperationWithMethod {
	operations := []struct {
		method    string
		operation *openAPIOperation
	}{
		{method: http.MethodDelete, operation: item.Delete},
		{method: http.MethodGet, operation: item.Get},
		{method: http.MethodHead, operation: item.Head},
		{method: http.MethodOptions, operation: item.Options},
		{method: http.MethodPatch, operation: item.Patch},
		{method: http.MethodPost, operation: item.Post},
		{method: http.MethodPut, operation: item.Put},
		{method: http.MethodTrace, operation: item.Trace},
	}

	result := make([]openAPIOperationWithMethod, 0, len(operations))
	for _, operation := range operations {
		if operation.operation != nil {
			result = append(result, openAPIOperationWithMethod{
				method:       operation.method,
				requestBody:  operation.operation.RequestBody,
				responseByID: operation.operation.Responses,
			})
		}
	}
	return result
}

func (operation openAPIOperationWithMethod) successResponseMediaTypes(components map[string]openAPIResponse) []string {
	mediaTypeSet := map[string]struct{}{}
	for status, response := range operation.responseByID {
		if len(status) != 3 || status[0] != '2' {
			continue
		}
		for mediaType := range resolvedResponse(response, components).Content {
			mediaType = strings.ToLower(strings.TrimSpace(mediaType))
			if mediaType != "" {
				mediaTypeSet[mediaType] = struct{}{}
			}
		}
	}

	result := make([]string, 0, len(mediaTypeSet))
	for mediaType := range mediaTypeSet {
		result = append(result, mediaType)
	}
	return result
}

func resolvedResponse(response openAPIResponse, components map[string]openAPIResponse) openAPIResponse {
	const prefix = "#/components/responses/"
	if !strings.HasPrefix(response.Ref, prefix) {
		return response
	}
	name := strings.TrimPrefix(response.Ref, prefix)
	if resolved, ok := components[name]; ok {
		return resolved
	}
	return response
}

var openAPIPathParameterPattern = regexp.MustCompile(`\{([^}/]+)\}`)

func openAPIPathToGinPath(path string) string {
	return "/api/v1" + openAPIPathParameterPattern.ReplaceAllString(path, ":$1")
}
