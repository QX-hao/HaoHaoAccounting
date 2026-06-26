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

func TestRegisteredHealthRoutesMatchHealthOpenAPIPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	if err := RegisterRoutesWithConfig(router, testutil.NewStore(t), nil, routeContractConfig()); err != nil {
		t.Fatalf("register routes: %v", err)
	}

	got := registeredNonAPIRoutes(router.Routes(), "/livez", "/readyz", "/health")
	want := healthOpenAPIRoutes(t)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("registered health routes = %#v, want health OpenAPI routes %#v", got, want)
	}
}

func TestHealthOpenAPIHeadResponsesDeclareNoBody(t *testing.T) {
	data := readHealthOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse health-openapi.yaml: %v", err)
	}

	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			if operation.method != http.MethodHead {
				continue
			}
			checked++
			for status, response := range operation.responses {
				resolved := resolvedResponse(response, doc.Components.Responses)
				if len(resolved.Content) != 0 {
					t.Fatalf("%s %s %s response must not declare a response body", operation.method, path, status)
				}
				for _, header := range []string{"Cache-Control", "Pragma", "Expires"} {
					if !resolvedResponseHasHeader(response, doc.Components.Responses, header) {
						t.Fatalf("%s %s %s response is missing %s header", operation.method, path, status, header)
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("health OpenAPI does not define any HEAD operations")
	}
}

func TestHealthOpenAPIRefsResolve(t *testing.T) {
	root := parseHealthOpenAPINode(t)

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
				t.Fatalf("health OpenAPI ref %s at line %d does not resolve", value.Value, value.Line)
			}
		}
	})
}

func TestHealthOpenAPIResponsesDeclareNoCacheHeaders(t *testing.T) {
	data := readHealthOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse health-openapi.yaml: %v", err)
	}

	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			for status, response := range operation.responses {
				checked++
				for _, header := range []string{"Cache-Control", "Pragma", "Expires"} {
					if !resolvedResponseHasHeader(response, doc.Components.Responses, header) {
						t.Fatalf("%s %s %s response is missing %s header", operation.method, path, status, header)
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("health OpenAPI does not define any responses")
	}
}

func TestHealthOpenAPISchemasAreClosed(t *testing.T) {
	data := readHealthOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse health-openapi.yaml: %v", err)
	}

	checked := 0
	for name, schema := range doc.Components.Schemas {
		if schema.Type != "object" {
			continue
		}
		checked++
		if schema.AdditionalProperties == nil || *schema.AdditionalProperties {
			t.Fatalf("health OpenAPI schema %s must set additionalProperties: false", name)
		}
	}
	if checked == 0 {
		t.Fatal("health OpenAPI does not define any object schemas")
	}
}

func TestHealthOpenAPIOperationsHaveStableDocumentation(t *testing.T) {
	data := readHealthOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse health-openapi.yaml: %v", err)
	}

	operationIDs := map[string]string{}
	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			checked++
			if operation.operationID == "" {
				t.Fatalf("%s %s is missing operationId", operation.method, path)
			}
			if operation.summary == "" {
				t.Fatalf("%s %s is missing summary", operation.method, path)
			}
			if operation.description == "" {
				t.Fatalf("%s %s is missing description", operation.method, path)
			}
			key := operation.method + " " + path
			if previous, ok := operationIDs[operation.operationID]; ok {
				t.Fatalf("operationId %s is used by both %s and %s", operation.operationID, previous, key)
			}
			operationIDs[operation.operationID] = key
		}
	}
	if checked == 0 {
		t.Fatal("health OpenAPI does not define any operations")
	}
}

func TestOpenAPIOperationsHaveStableDocumentation(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	operationIDs := map[string]string{}
	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			checked++
			if operation.operationID == "" {
				t.Fatalf("%s %s is missing operationId", operation.method, path)
			}
			if operation.summary == "" {
				t.Fatalf("%s %s is missing summary", operation.method, path)
			}
			if operation.description == "" {
				t.Fatalf("%s %s is missing description", operation.method, path)
			}
			key := operation.method + " " + path
			if previous, ok := operationIDs[operation.operationID]; ok {
				t.Fatalf("operationId %s is used by both %s and %s", operation.operationID, previous, key)
			}
			operationIDs[operation.operationID] = key
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any operations")
	}
}

func TestOpenAPISchemasAreClosed(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for name, schema := range doc.Components.Schemas {
		if schema.Type != "object" {
			continue
		}
		checked++
		if schema.AdditionalProperties == nil || *schema.AdditionalProperties {
			t.Fatalf("OpenAPI schema %s must set additionalProperties: false", name)
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any object schemas")
	}
}

func TestOpenAPIObjectSchemasDeclareProperties(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for name, schema := range doc.Components.Schemas {
		if schema.Type != "object" {
			continue
		}
		checked++
		if len(schema.Properties) == 0 {
			t.Fatalf("OpenAPI object schema %s must declare properties", name)
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any object schemas")
	}
}

func TestOpenAPIRequiredSchemaPropertiesExist(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for name, schema := range doc.Components.Schemas {
		if schema.Type != "object" {
			continue
		}
		for _, property := range schema.Required {
			checked++
			if _, ok := schema.Properties[property]; !ok {
				t.Fatalf("OpenAPI schema %s requires missing property %s", name, property)
			}
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any required schema properties")
	}
}

func TestOpenAPIArraySchemasDeclareItems(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for name, schema := range doc.Components.Schemas {
		checked += assertArraySchemasDeclareItems(t, schema, "#/components/schemas/"+name)
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any array schemas")
	}
}

func TestOpenAPIEnumSchemasAreNonEmpty(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for name, schema := range doc.Components.Schemas {
		checked += assertEnumSchemasAreNonEmpty(t, schema, "#/components/schemas/"+name)
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any enum schemas")
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

func TestOpenAPIRequestBodySchemasRejectAdditionalProperties(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			if operation.requestBody == nil {
				continue
			}
			for mediaType, media := range operation.requestBody.Content {
				ref := media.Schema.Ref
				if ref == "" {
					t.Fatalf("%s %s requestBody %s must use a component schema ref", operation.method, path, mediaType)
				}
				schema, ok := schemaByRef(doc.Components.Schemas, ref)
				if !ok {
					t.Fatalf("%s %s requestBody %s schema ref %s does not resolve", operation.method, path, mediaType, ref)
				}
				if schema.Type == "object" {
					checked++
					if schema.AdditionalProperties == nil || *schema.AdditionalProperties {
						t.Fatalf("%s %s requestBody %s schema %s must set additionalProperties: false", operation.method, path, mediaType, ref)
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any object requestBody schemas")
	}
}

func TestOpenAPIRequestBodyOperationsUseInvalidRequestResponse(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			if operation.requestBody == nil {
				continue
			}
			checked++
			response, ok := operation.responses["400"]
			if !ok {
				t.Fatalf("%s %s requestBody operation is missing 400 response", operation.method, path)
			}
			if response.Ref != "#/components/responses/InvalidRequest" {
				t.Fatalf("%s %s requestBody 400 response = %q, want InvalidRequest", operation.method, path, response.Ref)
			}
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any requestBody operations")
	}
}

func TestOpenAPIRequestBodyOperationsDeclareUnsupportedMediaTypeResponse(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			if operation.requestBody == nil {
				continue
			}
			checked++
			response, ok := operation.responses["415"]
			if !ok {
				t.Fatalf("%s %s requestBody operation is missing 415 response", operation.method, path)
			}
			if response.Ref != "#/components/responses/UnsupportedMediaType" {
				t.Fatalf("%s %s requestBody 415 response = %q, want UnsupportedMediaType", operation.method, path, response.Ref)
			}
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any requestBody operations")
	}
}

func TestOpenAPIQueryParameterOperationsUseInvalidRequestResponse(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			if !operation.hasQueryParameters(doc.Components.Parameters) {
				continue
			}
			checked++
			response, ok := operation.responses["400"]
			if !ok {
				t.Fatalf("%s %s query operation is missing 400 response", operation.method, path)
			}
			if response.Ref != "#/components/responses/InvalidRequest" {
				t.Fatalf("%s %s query 400 response = %q, want InvalidRequest", operation.method, path, response.Ref)
			}
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any query parameter operations")
	}
}

func TestOpenAPIPathParametersMatchPathTemplates(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for path, item := range doc.Paths {
		expected := openAPIPathParameterNames(path)
		for _, operation := range item.operations(doc.Components.Parameters) {
			declared := operation.pathParameters(doc.Components.Parameters)
			if !reflect.DeepEqual(declared, expected) {
				t.Fatalf("%s %s path parameters = %#v, want %#v", operation.method, path, declared, expected)
			}
			for _, parameter := range operation.resolvedParameters(doc.Components.Parameters) {
				if parameter.In != "path" {
					continue
				}
				checked++
				if !parameter.Required {
					t.Fatalf("%s %s path parameter %q must be required", operation.method, path, parameter.Name)
				}
				if parameter.Schema.Type != "integer" || parameter.Schema.Minimum == nil || *parameter.Schema.Minimum < 1 {
					t.Fatalf("%s %s path parameter %q must be a positive integer schema", operation.method, path, parameter.Name)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any path parameters")
	}
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

func TestOpenAPISuccessResponsesDeclareSharedRuntimeHeaders(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			for status, response := range operation.responses {
				if len(status) != 3 || status[0] != '2' {
					continue
				}
				checked++
				// 成功响应也要声明运行时公共响应头，客户端只看 OpenAPI 就能知道缓存和追踪语义。
				for _, header := range []string{"Cache-Control", "Pragma", "Expires", "X-Request-ID"} {
					if !resolvedResponseHasHeader(response, doc.Components.Responses, header) {
						t.Fatalf("%s %s %s response is missing %s header", operation.method, path, status, header)
					}
				}
				if len(resolvedResponse(response, doc.Components.Responses).Content) > 0 && !resolvedResponseHasHeader(response, doc.Components.Responses, "Vary") {
					t.Fatalf("%s %s %s response body is missing Vary header", operation.method, path, status)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any success responses")
	}
}

func TestOpenAPIResponseBodyOperationsDeclareNotAcceptableResponse(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			if !operation.hasSuccessResponseBody(doc.Components.Responses) {
				continue
			}
			checked++
			response, ok := operation.responses["406"]
			if !ok {
				t.Fatalf("%s %s response body operation is missing 406 response", operation.method, path)
			}
			if response.Ref != "#/components/responses/NotAcceptable" {
				t.Fatalf("%s %s response body 406 response = %q, want NotAcceptable", operation.method, path, response.Ref)
			}
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any response body operations")
	}
}

func TestOpenAPIMethodNotAllowedResponsesDeclareAllowHeader(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			response, ok := operation.responses["405"]
			if !ok {
				continue
			}
			checked++
			if !resolvedResponseHasHeader(response, doc.Components.Responses, "Allow") {
				t.Fatalf("%s %s 405 response is missing Allow header", operation.method, path)
			}
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any 405 responses")
	}
}

func TestOpenAPINotAcceptableResponsesDeclareVaryHeader(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			response, ok := operation.responses["406"]
			if !ok {
				continue
			}
			checked++
			if !resolvedResponseHasHeader(response, doc.Components.Responses, "Vary") {
				t.Fatalf("%s %s 406 response is missing Vary header", operation.method, path)
			}
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any 406 responses")
	}
}

func TestOpenAPILogoutSuccessDeclaresClearSiteDataHeader(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			if operation.operationID != "postAuthLogout" {
				continue
			}
			response, ok := operation.responses["200"]
			if !ok {
				t.Fatalf("%s %s is missing 200 response", operation.method, path)
			}
			if !resolvedResponseHasHeader(response, doc.Components.Responses, "Clear-Site-Data") {
				t.Fatalf("%s %s 200 response is missing Clear-Site-Data header", operation.method, path)
			}
			return
		}
	}
	t.Fatal("OpenAPI is missing postAuthLogout operation")
}

func TestOpenAPIInternalErrorOperationsDeclareContextErrorResponses(t *testing.T) {
	data := readOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}

	checked := 0
	for path, item := range doc.Paths {
		for _, operation := range item.operations(doc.Components.Parameters) {
			if _, ok := operation.responses["500"]; !ok {
				continue
			}
			checked++
			if _, ok := operation.responses["504"]; !ok {
				t.Fatalf("%s %s declares 500 but is missing 504 response", operation.method, path)
			}
			if _, ok := operation.responses["499"]; !ok {
				t.Fatalf("%s %s declares 500 but is missing 499 response", operation.method, path)
			}
		}
	}
	if checked == 0 {
		t.Fatal("OpenAPI does not define any 500 responses")
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
		for _, operation := range item.operations(doc.Components.Parameters) {
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

func registeredNonAPIRoutes(routes gin.RoutesInfo, paths ...string) []string {
	allowed := map[string]bool{}
	for _, path := range paths {
		allowed[path] = true
	}

	result := make([]string, 0, len(routes))
	for _, route := range routes {
		if !allowed[route.Path] {
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
		for _, method := range item.methods(doc.Components.Parameters) {
			result = append(result, method+" "+openAPIPathToGinPath(path))
		}
	}
	sort.Strings(result)
	return result
}

func healthOpenAPIRoutes(t *testing.T) []string {
	t.Helper()

	data := readHealthOpenAPI(t)
	var doc openAPIDocument
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse health-openapi.yaml: %v", err)
	}

	result := make([]string, 0, len(doc.Paths))
	for path, item := range doc.Paths {
		for _, method := range item.methods(doc.Components.Parameters) {
			result = append(result, method+" "+path)
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
		for _, operation := range item.operations(doc.Components.Parameters) {
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
		for _, operation := range item.operations(doc.Components.Parameters) {
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
		for _, operation := range item.operations(doc.Components.Parameters) {
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

func readHealthOpenAPI(t *testing.T) []byte {
	t.Helper()

	candidates := []string{
		filepath.Join("..", "..", "api", "health-openapi.yaml"),
		filepath.Join("backend", "api", "health-openapi.yaml"),
	}

	var lastErr error
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data
		}
		lastErr = err
	}
	t.Fatalf("read health-openapi.yaml from %v: %v", candidates, lastErr)
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

func parseHealthOpenAPINode(t *testing.T) *yaml.Node {
	t.Helper()

	var root yaml.Node
	if err := yaml.Unmarshal(readHealthOpenAPI(t), &root); err != nil {
		t.Fatalf("parse health-openapi.yaml: %v", err)
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
	Parameters      map[string]openAPIParameter      `yaml:"parameters"`
	Responses       map[string]openAPIResponse       `yaml:"responses"`
	Schemas         map[string]openAPISchema         `yaml:"schemas"`
}

type openAPISecurityScheme struct {
	Type         string `yaml:"type"`
	Scheme       string `yaml:"scheme"`
	BearerFormat string `yaml:"bearerFormat"`
}

type openAPIPathItem struct {
	Parameters []openAPIParameter `yaml:"parameters"`

	// 覆盖 OpenAPI Path Item 的标准 HTTP 操作，避免路由契约测试漏掉未来新增的方法。
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
	OperationID string                     `yaml:"operationId"`
	Summary     string                     `yaml:"summary"`
	Description string                     `yaml:"description"`
	Parameters  []openAPIParameter         `yaml:"parameters"`
	Security    []map[string][]string      `yaml:"security"`
	RequestBody *openAPIRequestBody        `yaml:"requestBody"`
	Responses   map[string]openAPIResponse `yaml:"responses"`
}

type openAPIParameter struct {
	Ref      string        `yaml:"$ref"`
	Name     string        `yaml:"name"`
	In       string        `yaml:"in"`
	Required bool          `yaml:"required"`
	Schema   openAPISchema `yaml:"schema"`
}

type openAPIRequestBody struct {
	Content map[string]openAPIMediaTypeRef `yaml:"content"`
}

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

type openAPISchema struct {
	Type                 string         `yaml:"type"`
	AdditionalProperties *bool          `yaml:"additionalProperties"`
	Minimum              *int           `yaml:"minimum"`
	Required             []string       `yaml:"required"`
	Properties           map[string]any `yaml:"properties"`
	Items                *openAPISchema `yaml:"items"`
	Enum                 []any          `yaml:"enum"`
}

type openAPIOperationWithMethod struct {
	method           string
	operationID      string
	summary          string
	description      string
	parameters       []openAPIParameter
	public           bool
	securityOverride bool
	requestBody      *openAPIRequestBody
	responses        map[string]openAPIResponse
}

func (item openAPIPathItem) methods(components map[string]openAPIParameter) []string {
	operations := item.operations(components)
	result := make([]string, 0, len(operations))
	for _, operation := range operations {
		result = append(result, operation.method)
	}
	return result
}

func (item openAPIPathItem) operations(components map[string]openAPIParameter) []openAPIOperationWithMethod {
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
				method:           operation.method,
				operationID:      operation.operation.OperationID,
				summary:          strings.TrimSpace(operation.operation.Summary),
				description:      strings.TrimSpace(operation.operation.Description),
				parameters:       mergedOpenAPIParameters(item.Parameters, operation.operation.Parameters, components),
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

func (operation openAPIOperationWithMethod) hasQueryParameters(components map[string]openAPIParameter) bool {
	for _, parameter := range operation.resolvedParameters(components) {
		if parameter.In == "query" {
			return true
		}
	}
	return false
}

func (operation openAPIOperationWithMethod) pathParameters(components map[string]openAPIParameter) []string {
	result := make([]string, 0)
	for _, parameter := range operation.resolvedParameters(components) {
		if parameter.In == "path" {
			result = append(result, parameter.Name)
		}
	}
	sort.Strings(result)
	return result
}

func (operation openAPIOperationWithMethod) resolvedParameters(components map[string]openAPIParameter) []openAPIParameter {
	result := make([]openAPIParameter, 0, len(operation.parameters))
	for _, parameter := range operation.parameters {
		result = append(result, resolvedParameter(parameter, components))
	}
	return result
}

// OpenAPI 允许 Path Item 声明公共参数；operation 同名同位置参数会覆盖公共参数。
func mergedOpenAPIParameters(pathParameters []openAPIParameter, operationParameters []openAPIParameter, components map[string]openAPIParameter) []openAPIParameter {
	if len(pathParameters) == 0 {
		return operationParameters
	}
	result := make([]openAPIParameter, 0, len(pathParameters)+len(operationParameters))
	indexes := make(map[string]int, len(pathParameters)+len(operationParameters))
	for _, parameter := range append(pathParameters, operationParameters...) {
		key := openAPIParameterKey(resolvedParameter(parameter, components))
		if index, ok := indexes[key]; ok {
			result[index] = parameter
			continue
		}
		indexes[key] = len(result)
		result = append(result, parameter)
	}
	return result
}

func openAPIParameterKey(parameter openAPIParameter) string {
	return parameter.In + ":" + parameter.Name
}

func resolvedParameter(parameter openAPIParameter, components map[string]openAPIParameter) openAPIParameter {
	const prefix = "#/components/parameters/"
	if !strings.HasPrefix(parameter.Ref, prefix) {
		return parameter
	}
	name := strings.TrimPrefix(parameter.Ref, prefix)
	if resolved, ok := components[name]; ok {
		return resolved
	}
	return parameter
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

func assertArraySchemasDeclareItems(t *testing.T, schema openAPISchema, path string) int {
	t.Helper()

	checked := 0
	if schema.Type == "array" {
		checked++
		if schema.Items == nil {
			t.Fatalf("OpenAPI array schema %s must declare items", path)
		}
		checked += assertArraySchemasDeclareItems(t, *schema.Items, path+".items")
	}
	for name, property := range schema.Properties {
		if child, ok := property.(map[string]any); ok {
			checked += assertArraySchemaMapDeclaresItems(t, child, path+".properties."+name)
		}
	}
	return checked
}

func assertArraySchemaMapDeclaresItems(t *testing.T, schema map[string]any, path string) int {
	t.Helper()

	checked := 0
	if schema["type"] == "array" {
		checked++
		if _, ok := schema["items"]; !ok {
			t.Fatalf("OpenAPI array schema %s must declare items", path)
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		checked += assertArraySchemaMapDeclaresItems(t, items, path+".items")
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for name, property := range properties {
			if child, ok := property.(map[string]any); ok {
				checked += assertArraySchemaMapDeclaresItems(t, child, path+".properties."+name)
			}
		}
	}
	return checked
}

func assertEnumSchemasAreNonEmpty(t *testing.T, schema openAPISchema, path string) int {
	t.Helper()

	checked := 0
	if schema.Enum != nil {
		checked++
		if len(schema.Enum) == 0 {
			t.Fatalf("OpenAPI enum schema %s must list at least one value", path)
		}
	}
	for name, property := range schema.Properties {
		if child, ok := property.(map[string]any); ok {
			checked += assertEnumSchemaMapIsNonEmpty(t, child, path+".properties."+name)
		}
	}
	if schema.Items != nil {
		checked += assertEnumSchemasAreNonEmpty(t, *schema.Items, path+".items")
	}
	return checked
}

func assertEnumSchemaMapIsNonEmpty(t *testing.T, schema map[string]any, path string) int {
	t.Helper()

	checked := 0
	if rawEnum, ok := schema["enum"]; ok {
		checked++
		values, ok := rawEnum.([]any)
		if !ok || len(values) == 0 {
			t.Fatalf("OpenAPI enum schema %s must list at least one value", path)
		}
	}
	if items, ok := schema["items"].(map[string]any); ok {
		checked += assertEnumSchemaMapIsNonEmpty(t, items, path+".items")
	}
	if properties, ok := schema["properties"].(map[string]any); ok {
		for name, property := range properties {
			if child, ok := property.(map[string]any); ok {
				checked += assertEnumSchemaMapIsNonEmpty(t, child, path+".properties."+name)
			}
		}
	}
	return checked
}

func schemaByRef(schemas map[string]openAPISchema, ref string) (openAPISchema, bool) {
	const prefix = "#/components/schemas/"
	if !strings.HasPrefix(ref, prefix) {
		return openAPISchema{}, false
	}
	schema, ok := schemas[strings.TrimPrefix(ref, prefix)]
	return schema, ok
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

func openAPIPathParameterNames(path string) []string {
	matches := openAPIPathParameterPattern.FindAllStringSubmatch(path, -1)
	result := make([]string, 0, len(matches))
	for _, match := range matches {
		result = append(result, match[1])
	}
	sort.Strings(result)
	return result
}

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
