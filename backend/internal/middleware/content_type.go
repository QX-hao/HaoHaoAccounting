package middleware

import (
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/gin-gonic/gin"
)

type ContentTypeRule struct {
	Method       string
	Path         string
	AllowedTypes []string
}

// ContentType 按路由声明的请求体媒体类型检查 Content-Type，避免 handler 处理未声明格式。
func ContentType(rules []ContentTypeRule) gin.HandlerFunc {
	lookup := make(map[string][]string, len(rules))
	for _, rule := range rules {
		method := strings.ToUpper(strings.TrimSpace(rule.Method))
		path := strings.TrimSpace(rule.Path)
		if method == "" || path == "" || len(rule.AllowedTypes) == 0 {
			continue
		}
		if allowed := normalizeMediaTypes(rule.AllowedTypes); len(allowed) > 0 {
			lookup[method+" "+path] = allowed
		}
	}

	return func(c *gin.Context) {
		allowed, ok := lookup[c.Request.Method+" "+c.FullPath()]
		if !ok {
			c.Next()
			return
		}

		headerValues := c.Request.Header.Values("Content-Type")
		contentType := ""
		if len(headerValues) == 1 {
			contentType = strings.TrimSpace(headerValues[0])
		}
		mediaType, params, err := mime.ParseMediaType(contentType)
		if contentType == "" || err != nil || !mediaTypeAllowed(mediaType, allowed) || !mediaTypeParametersAllowed(mediaType, params) {
			httputil.UnsupportedMediaType(c, unsupportedMediaTypeMessage(allowed))
			c.Abort()
			return
		}
		c.Next()
	}
}

func normalizeMediaTypes(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		clean := strings.ToLower(strings.TrimSpace(value))
		mediaTypeValue, params, err := mime.ParseMediaType(clean)
		if err != nil || len(params) > 0 || mediaTypeValue != clean {
			continue
		}
		mediaType, mediaSubtype, ok := splitMediaType(clean)
		if !ok || mediaType == "*" || mediaSubtype == "*" || strings.HasPrefix(mediaSubtype, "*+") {
			continue
		}
		if _, exists := seen[clean]; exists {
			continue
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}
	return result
}

func mediaTypeAllowed(mediaType string, allowed []string) bool {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	for _, allowedType := range allowed {
		if mediaType == allowedType || structuredJSONMediaTypeAllowed(mediaType, allowedType) {
			return true
		}
	}
	return false
}

func mediaTypeParametersAllowed(mediaType string, params map[string]string) bool {
	// multipart/form-data 没有 boundary 时，后续 multipart reader 无法可靠解析表单内容。
	if strings.EqualFold(strings.TrimSpace(mediaType), "multipart/form-data") {
		return strings.TrimSpace(params["boundary"]) != ""
	}
	return true
}

func structuredJSONMediaTypeAllowed(mediaType string, allowedType string) bool {
	if allowedType != "application/json" {
		return false
	}
	return strings.HasPrefix(mediaType, "application/") && strings.HasSuffix(mediaType, "+json")
}

func unsupportedMediaTypeMessage(allowed []string) string {
	return fmt.Sprintf("unsupported media type: expected %s", strings.Join(allowed, " or "))
}

// APIMediaTypeRules 从当前 API/OpenAPI 契约整理有请求体路由的 Content-Type 白名单。
func APIMediaTypeRules() []ContentTypeRule {
	return []ContentTypeRule{
		{Method: http.MethodPost, Path: "/api/v1/auth/login", AllowedTypes: []string{"application/json"}},
		{Method: http.MethodPost, Path: "/api/v1/accounts", AllowedTypes: []string{"application/json"}},
		{Method: http.MethodPut, Path: "/api/v1/accounts/:id", AllowedTypes: []string{"application/json"}},
		{Method: http.MethodPost, Path: "/api/v1/budgets", AllowedTypes: []string{"application/json"}},
		{Method: http.MethodPut, Path: "/api/v1/budgets/:id", AllowedTypes: []string{"application/json"}},
		{Method: http.MethodPost, Path: "/api/v1/categories", AllowedTypes: []string{"application/json"}},
		{Method: http.MethodPut, Path: "/api/v1/categories/:id", AllowedTypes: []string{"application/json"}},
		{Method: http.MethodPost, Path: "/api/v1/transactions", AllowedTypes: []string{"application/json"}},
		{Method: http.MethodPut, Path: "/api/v1/transactions/:id", AllowedTypes: []string{"application/json"}},
		{Method: http.MethodPost, Path: "/api/v1/ai/parse", AllowedTypes: []string{"application/json"}},
		{Method: http.MethodPost, Path: "/api/v1/io/import/text/preview", AllowedTypes: []string{"application/json"}},
		{Method: http.MethodPost, Path: "/api/v1/io/import/text", AllowedTypes: []string{"application/json"}},
		{Method: http.MethodPost, Path: "/api/v1/io/import/preview", AllowedTypes: []string{"multipart/form-data"}},
		{Method: http.MethodPost, Path: "/api/v1/io/import", AllowedTypes: []string{"multipart/form-data"}},
		{Method: http.MethodPost, Path: "/api/v1/io/import/jobs", AllowedTypes: []string{"multipart/form-data"}},
	}
}
