package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/config"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/testutil"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

func TestRegisteredAPIRoutesMatchOpenAPIPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	if err := RegisterRoutesWithConfig(router, testutil.NewStore(t), nil, routeContractConfig()); err != nil {
		t.Fatalf("register routes: %v", err)
	}

	got := registeredAPIRoutes(router.Routes())
	want := openAPIRoutes(t)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registered API routes = %#v, want OpenAPI routes %#v", got, want)
	}
}

func TestRegisteredPrivateAPIRoutesRequireBearerAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	if err := RegisterRoutesWithConfig(router, testutil.NewStore(t), nil, routeContractConfig()); err != nil {
		t.Fatalf("register routes: %v", err)
	}

	publicRoutes := openAPIPublicRoutes(t)
	for _, route := range router.Routes() {
		if !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}
		key := route.Method + " " + route.Path
		if publicRoutes[key] {
			continue
		}

		t.Run(key, func(t *testing.T) {
			req := httptest.NewRequest(route.Method, pathWithExampleParams(route.Path), nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != http.StatusUnauthorized {
				t.Fatalf("%s without bearer auth status = %d, want 401, body = %s", key, resp.Code, resp.Body.String())
			}
		})
	}
}

func TestOpenAPIPublicRoutesMatchRegisteredPublicRoutes(t *testing.T) {
	got := sortedMapKeys(openAPIPublicRoutes(t))
	want := []string{http.MethodPost + " /api/v1/auth/login"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("OpenAPI public routes = %#v, want %#v", got, want)
	}
}

func routeContractConfig() config.Config {
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

func registeredAPIRoutes(routes gin.RoutesInfo) []string {
	result := make([]string, 0, len(routes))
	for _, route := range routes {
		if !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}
		result = append(result, route.Method+" "+route.Path)
	}
	sort.Strings(result)
	return result
}

func openAPIRoutes(t *testing.T) []string {
	t.Helper()

	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	result := make([]string, 0, len(doc.Paths))
	for path, item := range doc.Paths {
		for _, method := range item.methods() {
			result = append(result, method+" "+openAPIPathToGinPath(path))
		}
	}
	sort.Strings(result)
	return result
}

func openAPIPublicRoutes(t *testing.T) map[string]bool {
	t.Helper()

	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	result := map[string]bool{}
	for path, item := range doc.Paths {
		for _, operation := range item.operations() {
			if operation.public {
				result[operation.method+" "+openAPIPathToGinPath(path)] = true
			}
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
	Paths map[string]openAPIPathItem `yaml:"paths"`
}

type openAPIPathItem struct {
	Delete *openAPIOperation `yaml:"delete"`
	Get    *openAPIOperation `yaml:"get"`
	Patch  *openAPIOperation `yaml:"patch"`
	Post   *openAPIOperation `yaml:"post"`
	Put    *openAPIOperation `yaml:"put"`
}

type openAPIOperation struct {
	Security []map[string][]string `yaml:"security"`
}

type openAPIOperationWithMethod struct {
	method string
	public bool
}

func (item openAPIPathItem) methods() []string {
	operations := item.operations()
	result := make([]string, 0, len(operations))
	for _, operation := range operations {
		result = append(result, operation.method)
	}
	return result
}

func (item openAPIPathItem) operations() []openAPIOperationWithMethod {
	operations := []struct {
		method    string
		operation *openAPIOperation
	}{
		{method: http.MethodDelete, operation: item.Delete},
		{method: http.MethodGet, operation: item.Get},
		{method: http.MethodPatch, operation: item.Patch},
		{method: http.MethodPost, operation: item.Post},
		{method: http.MethodPut, operation: item.Put},
	}

	result := make([]openAPIOperationWithMethod, 0, len(operations))
	for _, operation := range operations {
		if operation.operation != nil {
			result = append(result, openAPIOperationWithMethod{
				method: operation.method,
				public: operation.operation.isPublic(),
			})
		}
	}
	return result
}

func (operation openAPIOperation) isPublic() bool {
	return operation.Security != nil && len(operation.Security) == 0
}

var openAPIPathParameterPattern = regexp.MustCompile(`\{([^}/]+)\}`)

func openAPIPathToGinPath(path string) string {
	return "/api/v1" + openAPIPathParameterPattern.ReplaceAllString(path, ":$1")
}

func pathWithExampleParams(path string) string {
	return strings.ReplaceAll(path, ":id", "1")
}

func sortedMapKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
