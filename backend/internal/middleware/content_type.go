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

func ContentType(rules []ContentTypeRule) gin.HandlerFunc {
	lookup := make(map[string][]string, len(rules))
	for _, rule := range rules {
		method := strings.ToUpper(strings.TrimSpace(rule.Method))
		path := strings.TrimSpace(rule.Path)
		if method == "" || path == "" || len(rule.AllowedTypes) == 0 {
			continue
		}
		lookup[method+" "+path] = normalizeMediaTypes(rule.AllowedTypes)
	}

	return func(c *gin.Context) {
		allowed, ok := lookup[c.Request.Method+" "+c.FullPath()]
		if !ok {
			c.Next()
			return
		}

		contentType := strings.TrimSpace(c.GetHeader("Content-Type"))
		mediaType, _, err := mime.ParseMediaType(contentType)
		if contentType == "" || err != nil || !mediaTypeAllowed(mediaType, allowed) {
			httputil.UnsupportedMediaType(c, unsupportedMediaTypeMessage(allowed))
			c.Abort()
			return
		}
		c.Next()
	}
}

func normalizeMediaTypes(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if clean := strings.ToLower(strings.TrimSpace(value)); clean != "" {
			result = append(result, clean)
		}
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

func structuredJSONMediaTypeAllowed(mediaType string, allowedType string) bool {
	if allowedType != "application/json" {
		return false
	}
	return strings.HasPrefix(mediaType, "application/") && strings.HasSuffix(mediaType, "+json")
}

func unsupportedMediaTypeMessage(allowed []string) string {
	return fmt.Sprintf("unsupported media type: expected %s", strings.Join(allowed, " or "))
}

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
