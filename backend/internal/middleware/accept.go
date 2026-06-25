package middleware

import (
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/gin-gonic/gin"
)

type AcceptRule struct {
	Method       string
	Path         string
	OfferedTypes []string
	Offered      func(*gin.Context) []string
}

// Accept 按路由声明的响应媒体类型检查 Accept 头，拒绝无法协商的请求并补充 Vary: Accept。
func Accept(rules []AcceptRule) gin.HandlerFunc {
	lookup := make(map[string][]string, len(rules))
	for _, rule := range rules {
		method := strings.ToUpper(strings.TrimSpace(rule.Method))
		path := strings.TrimSpace(rule.Path)
		if method == "" || path == "" || len(rule.OfferedTypes) == 0 {
			continue
		}
		if offered := normalizeMediaTypes(rule.OfferedTypes); len(offered) > 0 {
			lookup[method+" "+path] = offered
		}
	}

	return func(c *gin.Context) {
		key := c.Request.Method + " " + c.FullPath()
		offered, ok := lookup[key]
		if !ok {
			c.Next()
			return
		}
		for _, rule := range rules {
			method := strings.ToUpper(strings.TrimSpace(rule.Method))
			path := strings.TrimSpace(rule.Path)
			if method+" "+path == key && rule.Offered != nil {
				if dynamicOffered := normalizeMediaTypes(rule.Offered(c)); len(dynamicOffered) > 0 {
					offered = dynamicOffered
				}
				break
			}
		}
		appendVary(c.Writer.Header(), "Accept")

		if !acceptsAnyOfferedType(c.GetHeader("Accept"), offered) {
			httputil.NotAcceptable(c, notAcceptableMessage(offered))
			c.Abort()
			return
		}
		c.Next()
	}
}

func acceptsAnyOfferedType(header string, offered []string) bool {
	header = strings.TrimSpace(header)
	if header == "" {
		return true
	}

	ranges := parseAcceptMediaRanges(header)
	for _, offeredType := range offered {
		if mediaTypeAccepted(offeredType, ranges) {
			return true
		}
	}
	return false
}

type acceptMediaRange struct {
	mediaRange  string
	quality     float64
	specificity int
}

func parseAcceptMediaRanges(header string) []acceptMediaRange {
	result := make([]acceptMediaRange, 0)
	for _, item := range strings.Split(header, ",") {
		mediaRange, params, err := mime.ParseMediaType(strings.TrimSpace(item))
		if err != nil {
			continue
		}
		mediaRange = strings.ToLower(strings.TrimSpace(mediaRange))
		if _, _, ok := splitMediaType(mediaRange); !ok {
			continue
		}
		result = append(result, acceptMediaRange{
			mediaRange:  mediaRange,
			quality:     mediaQuality(params),
			specificity: mediaRangeSpecificity(mediaRange),
		})
	}
	return result
}

func mediaQuality(params map[string]string) float64 {
	raw := strings.TrimSpace(params["q"])
	if raw == "" {
		return 1
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0
	}
	if value < 0 || value > 1 {
		return 0
	}
	return value
}

func mediaTypeAccepted(offeredType string, ranges []acceptMediaRange) bool {
	bestSpecificity := -1
	bestQuality := 0.0
	for _, item := range ranges {
		if !mediaRangeMatches(item.mediaRange, offeredType) {
			continue
		}
		if item.specificity > bestSpecificity {
			bestSpecificity = item.specificity
			bestQuality = item.quality
			continue
		}
		if item.specificity == bestSpecificity && item.quality > bestQuality {
			bestQuality = item.quality
		}
	}
	return bestSpecificity >= 0 && bestQuality > 0
}

func mediaRangeSpecificity(mediaRange string) int {
	rangeType, rangeSubtype, ok := splitMediaType(mediaRange)
	if !ok {
		return -1
	}
	if rangeType == "*" && rangeSubtype == "*" {
		return 0
	}
	if rangeSubtype == "*" {
		return 1
	}
	if strings.HasPrefix(rangeSubtype, "*+") {
		return 2
	}
	return 3
}

func mediaRangeMatches(mediaRange string, offeredType string) bool {
	if mediaRange == "*/*" || mediaRange == offeredType {
		return true
	}
	rangeType, rangeSubtype, ok := splitMediaType(mediaRange)
	if !ok {
		return false
	}
	offeredTypePart, offeredSubtype, ok := splitMediaType(offeredType)
	if !ok {
		return false
	}
	if rangeType == offeredTypePart && structuredSyntaxSuffixMatches(rangeSubtype, offeredSubtype) {
		return true
	}
	return rangeType == offeredTypePart && rangeSubtype == "*"
}

func splitMediaType(value string) (string, string, bool) {
	parts := strings.Split(value, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func structuredSyntaxSuffixMatches(rangeSubtype string, offeredSubtype string) bool {
	if !strings.HasPrefix(rangeSubtype, "*+") {
		return false
	}
	suffix := strings.TrimPrefix(rangeSubtype, "*+")
	return suffix != "" && strings.HasSuffix(offeredSubtype, "+"+suffix)
}

func notAcceptableMessage(offered []string) string {
	return fmt.Sprintf("not acceptable: expected %s", strings.Join(offered, " or "))
}

func appendVary(headers http.Header, field string) {
	field = strings.TrimSpace(field)
	if field == "" {
		return
	}

	current := headers.Get("Vary")
	if current == "" {
		headers.Set("Vary", field)
		return
	}
	for _, value := range strings.Split(current, ",") {
		value = strings.TrimSpace(value)
		if value == "*" || strings.EqualFold(value, field) {
			return
		}
	}
	headers.Set("Vary", current+", "+field)
}

// APIAcceptRules 从当前 API/OpenAPI 契约整理响应媒体类型规则，文件下载接口按 format 动态收窄类型。
func APIAcceptRules() []AcceptRule {
	rules := []AcceptRule{
		{
			Method:       http.MethodGet,
			Path:         "/api/v1/io/export",
			OfferedTypes: []string{"text/csv", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
			Offered: func(c *gin.Context) []string {
				if strings.TrimSpace(c.Query("format")) == "xlsx" {
					return []string{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}
				}
				return []string{"text/csv"}
			},
		},
	}
	for _, route := range apiJSONRoutes() {
		rules = append(rules, AcceptRule{
			Method:       route.Method,
			Path:         route.Path,
			OfferedTypes: []string{"application/json"},
		})
	}
	return rules
}

type apiRoute struct {
	Method string
	Path   string
}

func apiJSONRoutes() []apiRoute {
	return []apiRoute{
		{Method: http.MethodPost, Path: "/api/v1/auth/login"},
		{Method: http.MethodPost, Path: "/api/v1/auth/refresh"},
		{Method: http.MethodPost, Path: "/api/v1/auth/logout"},
		{Method: http.MethodGet, Path: "/api/v1/me"},
		{Method: http.MethodGet, Path: "/api/v1/accounts"},
		{Method: http.MethodPost, Path: "/api/v1/accounts"},
		{Method: http.MethodPut, Path: "/api/v1/accounts/:id"},
		{Method: http.MethodDelete, Path: "/api/v1/accounts/:id"},
		{Method: http.MethodGet, Path: "/api/v1/budgets"},
		{Method: http.MethodPost, Path: "/api/v1/budgets"},
		{Method: http.MethodPut, Path: "/api/v1/budgets/:id"},
		{Method: http.MethodDelete, Path: "/api/v1/budgets/:id"},
		{Method: http.MethodGet, Path: "/api/v1/categories"},
		{Method: http.MethodPost, Path: "/api/v1/categories"},
		{Method: http.MethodPut, Path: "/api/v1/categories/:id"},
		{Method: http.MethodDelete, Path: "/api/v1/categories/:id"},
		{Method: http.MethodGet, Path: "/api/v1/transactions"},
		{Method: http.MethodPost, Path: "/api/v1/transactions"},
		{Method: http.MethodPut, Path: "/api/v1/transactions/:id"},
		{Method: http.MethodDelete, Path: "/api/v1/transactions/:id"},
		{Method: http.MethodPost, Path: "/api/v1/ai/parse"},
		{Method: http.MethodGet, Path: "/api/v1/reports/summary"},
		{Method: http.MethodPost, Path: "/api/v1/io/import/preview"},
		{Method: http.MethodPost, Path: "/api/v1/io/import"},
		{Method: http.MethodPost, Path: "/api/v1/io/import/jobs"},
		{Method: http.MethodGet, Path: "/api/v1/io/import/jobs"},
		{Method: http.MethodGet, Path: "/api/v1/io/import/jobs/:id"},
		{Method: http.MethodPost, Path: "/api/v1/io/import/text/preview"},
		{Method: http.MethodPost, Path: "/api/v1/io/import/text"},
	}
}
