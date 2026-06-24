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
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
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

func TestRegisteredResponseBodyRoutesHaveAcceptRules(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	if err := RegisterRoutesWithConfig(router, testutil.NewStore(t), nil, routeContractConfig()); err != nil {
		t.Fatalf("register routes: %v", err)
	}

	assertRegisteredRoutesHaveRules(t, router.Routes(), openAPIResponseBodyRoutes(t), acceptRuleSet(middleware.APIAcceptRules()), "Accept")
}

func TestRegisteredRequestBodyRoutesHaveContentTypeRules(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	if err := RegisterRoutesWithConfig(router, testutil.NewStore(t), nil, routeContractConfig()); err != nil {
		t.Fatalf("register routes: %v", err)
	}

	assertRegisteredRoutesHaveRules(t, router.Routes(), openAPIRequestBodyRoutes(t), contentTypeRuleSet(middleware.APIMediaTypeRules()), "Content-Type")
}

func assertRegisteredRoutesHaveRules(t *testing.T, routes gin.RoutesInfo, constrainedRoutes map[string]bool, rules map[string]bool, ruleName string) {
	t.Helper()

	for _, route := range routes {
		if !strings.HasPrefix(route.Path, "/api/v1/") {
			continue
		}
		key := route.Method + " " + route.Path
		if !constrainedRoutes[key] {
			continue
		}
		if !rules[key] {
			t.Fatalf("registered route %s is missing an API %s rule", key, ruleName)
		}
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

func TestOpenAPIErrorResponsesUseSharedHeadersAndSchema(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for name, response := range doc.Components.Responses {
		media, ok := response.Content["application/json"]
		if !ok || media.Schema.Ref != "#/components/schemas/ErrorResponse" {
			continue
		}
		checked++
		for _, header := range []string{"Cache-Control", "Pragma", "Expires", "X-Request-ID"} {
			if _, ok := response.Headers[header]; !ok {
				t.Fatalf("OpenAPI error response %s is missing %s header", name, header)
			}
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any shared ErrorResponse components")
	}
}

func TestOpenAPISecurityContractMatchesBearerMiddleware(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	assertBearerSecurityScheme(t, doc.Components.SecuritySchemes["bearerAuth"])
	assertBearerSecurityRequirement(t, doc.Security, "OpenAPI root security")

	publicRoutes := map[string]bool{}
	for path, item := range doc.Paths {
		for _, operation := range item.operations() {
			key := operation.method + " " + openAPIPathToGinPath(path)
			if operation.public {
				publicRoutes[key] = true
				continue
			}

			if operation.securityOverride {
				t.Fatalf("private route %s must inherit root bearerAuth security instead of overriding it", key)
			}
			if response, ok := operation.responses["401"]; !ok {
				t.Fatalf("private route %s is missing a 401 response", key)
			} else if !resolvedResponseHasHeader(response, doc.Components.Responses, "WWW-Authenticate") {
				t.Fatalf("private route %s 401 response is missing WWW-Authenticate header", key)
			}
		}
	}

	wantPublicRoutes := []string{http.MethodPost + " /api/v1/auth/login"}
	if got := sortedMapKeys(publicRoutes); !reflect.DeepEqual(got, wantPublicRoutes) {
		t.Fatalf("OpenAPI public routes = %#v, want %#v", got, wantPublicRoutes)
	}
}

func TestOpenAPIInternalRefsResolve(t *testing.T) {
	root := parseOpenAPINode(t)

	walkOpenAPINode(root, func(node *yaml.Node) {
		if node.Kind != yaml.MappingNode {
			return
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if key.Value != "$ref" || !strings.HasPrefix(value.Value, "#/") {
				continue
			}
			if !openAPIRefExists(root, value.Value) {
				t.Fatalf("OpenAPI ref %s at line %d does not resolve", value.Value, value.Line)
			}
		}
	})
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

func openAPIResponseBodyRoutes(t *testing.T) map[string]bool {
	t.Helper()

	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	result := map[string]bool{}
	for path, item := range doc.Paths {
		for _, operation := range item.operations() {
			if operation.hasSuccessResponseBody(doc.Components.Responses) {
				result[operation.method+" "+openAPIPathToGinPath(path)] = true
			}
		}
	}
	return result
}

func openAPIRequestBodyRoutes(t *testing.T) map[string]bool {
	t.Helper()

	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	result := map[string]bool{}
	for path, item := range doc.Paths {
		for _, operation := range item.operations() {
			if operation.requestBody != nil {
				result[operation.method+" "+openAPIPathToGinPath(path)] = true
			}
		}
	}
	return result
}

func acceptRuleSet(rules []middleware.AcceptRule) map[string]bool {
	result := map[string]bool{}
	for _, rule := range rules {
		method := strings.ToUpper(strings.TrimSpace(rule.Method))
		path := strings.TrimSpace(rule.Path)
		if method != "" && path != "" {
			result[method+" "+path] = true
		}
	}
	return result
}

func contentTypeRuleSet(rules []middleware.ContentTypeRule) map[string]bool {
	result := map[string]bool{}
	for _, rule := range rules {
		method := strings.ToUpper(strings.TrimSpace(rule.Method))
		path := strings.TrimSpace(rule.Path)
		if method != "" && path != "" {
			result[method+" "+path] = true
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

func parseOpenAPINode(t *testing.T) *yaml.Node {
	t.Helper()

	var root yaml.Node
	if err := yaml.Unmarshal(readOpenAPI(t), &root); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		return root.Content[0]
	}
	return &root
}

func walkOpenAPINode(node *yaml.Node, visit func(*yaml.Node)) {
	if node == nil {
		return
	}
	visit(node)
	for _, child := range node.Content {
		walkOpenAPINode(child, visit)
	}
}

func openAPIRefExists(root *yaml.Node, ref string) bool {
	node := root
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		next := mappingValue(node, part)
		if next == nil {
			return false
		}
		node = next
	}
	return true
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

type openAPIDocument struct {
	Security   []map[string][]string      `yaml:"security"`
	Paths      map[string]openAPIPathItem `yaml:"paths"`
	Components openAPIComponents          `yaml:"components"`
}

type openAPIComponents struct {
	SecuritySchemes map[string]openAPISecurityScheme `yaml:"securitySchemes"`
	Responses       map[string]openAPIResponse       `yaml:"responses"`
}

type openAPISecurityScheme struct {
	Type         string `yaml:"type"`
	Scheme       string `yaml:"scheme"`
	BearerFormat string `yaml:"bearerFormat"`
}

type openAPIPathItem struct {
	Delete *openAPIOperation `yaml:"delete"`
	Get    *openAPIOperation `yaml:"get"`
	Patch  *openAPIOperation `yaml:"patch"`
	Post   *openAPIOperation `yaml:"post"`
	Put    *openAPIOperation `yaml:"put"`
}

type openAPIOperation struct {
	Security    []map[string][]string      `yaml:"security"`
	RequestBody *openAPIRequestBody        `yaml:"requestBody"`
	Responses   map[string]openAPIResponse `yaml:"responses"`
}

type openAPIRequestBody struct{}

type openAPIResponse struct {
	Ref     string                         `yaml:"$ref"`
	Headers map[string]any                 `yaml:"headers"`
	Content map[string]openAPIMediaTypeRef `yaml:"content"`
}

type openAPIMediaTypeRef struct {
	Schema openAPIRef `yaml:"schema"`
}

type openAPIRef struct {
	Ref string `yaml:"$ref"`
}

type openAPIOperationWithMethod struct {
	method           string
	public           bool
	securityOverride bool
	requestBody      *openAPIRequestBody
	responses        map[string]openAPIResponse
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
				method:           operation.method,
				public:           operation.operation.isPublic(),
				securityOverride: operation.operation.Security != nil,
				requestBody:      operation.operation.RequestBody,
				responses:        operation.operation.Responses,
			})
		}
	}
	return result
}

func (operation openAPIOperationWithMethod) hasSuccessResponseBody(components map[string]openAPIResponse) bool {
	for status, response := range operation.responses {
		if len(status) != 3 || status[0] != '2' {
			continue
		}
		if len(resolvedResponse(response, components).Content) > 0 {
			return true
		}
	}
	return false
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

func resolvedResponseHasHeader(response openAPIResponse, components map[string]openAPIResponse, header string) bool {
	_, ok := resolvedResponse(response, components).Headers[header]
	return ok
}

func (operation openAPIOperation) isPublic() bool {
	return operation.Security != nil && len(operation.Security) == 0
}

func assertBearerSecurityScheme(t *testing.T, scheme openAPISecurityScheme) {
	t.Helper()

	if scheme.Type != "http" || scheme.Scheme != "bearer" || scheme.BearerFormat != "JWT" {
		t.Fatalf("bearerAuth scheme = %#v, want HTTP bearer JWT", scheme)
	}
}

func assertBearerSecurityRequirement(t *testing.T, security []map[string][]string, name string) {
	t.Helper()

	if len(security) != 1 {
		t.Fatalf("%s = %#v, want exactly one bearerAuth requirement", name, security)
	}
	scopes, ok := security[0]["bearerAuth"]
	if !ok {
		t.Fatalf("%s = %#v, missing bearerAuth", name, security)
	}
	if scopes == nil {
		return
	}
	if len(scopes) != 0 {
		t.Fatalf("%s bearerAuth scopes = %#v, want empty scopes for HTTP bearer auth", name, scopes)
	}
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
