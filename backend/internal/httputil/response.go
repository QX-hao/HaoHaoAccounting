package httputil

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

const (
	CodeBadRequest           = "bad_request"
	CodeUnauthorized         = "unauthorized"
	CodeForbidden            = "forbidden"
	CodeNotFound             = "not_found"
	CodeMethodNotAllowed     = "method_not_allowed"
	CodeRateLimited          = "rate_limited"
	CodePayloadTooLarge      = "payload_too_large"
	CodeUnsupportedMediaType = "unsupported_media_type"
	CodeNotAcceptable        = "not_acceptable"
	CodeRequestTimeout       = "request_timeout"
	CodeClientClosedRequest  = "client_closed_request"
	CodeInternal             = "internal_error"
	CodeInvalidRequest       = "invalid_request"
)

const StatusClientClosedRequest = 499

const requestIDContextKey = "request_id"

const (
	bearerChallenge             = `Bearer realm="haohao-accounting-api"`
	invalidBearerTokenChallenge = bearerChallenge + `, error="invalid_token", error_description="The access token is missing, expired, revoked, or invalid"`
)

type ErrorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	Status    int    `json:"status"`
	RequestID string `json:"requestId"`
}

// Error 写入统一错误 envelope，并把不含用户输入的 status/code 摘要挂到 Gin 私有错误日志。
func Error(c *gin.Context, status int, code string, message string) {
	if c.Writer.Written() {
		return
	}
	if status < http.StatusBadRequest || status > 599 {
		status = http.StatusInternalServerError
		code = CodeInternal
		message = "internal server error"
	}
	if code == "" {
		code = codeForStatus(status)
	}
	_ = c.Error(responseLogError{status: status, code: code})
	c.JSON(status, ErrorResponse{
		Error:     message,
		Code:      code,
		Status:    status,
		RequestID: requestIDFromContext(c),
	})
}

// BadRequest 返回通用 400 错误，适合参数或业务校验失败。
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, CodeBadRequest, message)
}

// InvalidRequest 返回 400 invalid_request，适合请求体格式或绑定失败。
func InvalidRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, CodeInvalidRequest, message)
}

// Unauthorized 返回 401 并写入基础 Bearer challenge。
func Unauthorized(c *gin.Context, message string) {
	c.Header("WWW-Authenticate", bearerChallenge)
	Error(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

// InvalidToken 返回 401 并写入 RFC 6750 invalid_token challenge。
func InvalidToken(c *gin.Context, message string) {
	c.Header("WWW-Authenticate", invalidBearerTokenChallenge)
	Error(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

// Forbidden 返回已认证但无权限的 403 错误。
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, CodeForbidden, message)
}

// NotFound 返回资源或路由不存在的 404 错误。
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, CodeNotFound, message)
}

// MethodNotAllowed 返回 405 错误，调用方应同时补充 Allow 响应头。
func MethodNotAllowed(c *gin.Context, message string) {
	Error(c, http.StatusMethodNotAllowed, CodeMethodNotAllowed, message)
}

// RateLimited 返回 429，并在有等待时间时写入 Retry-After 秒数。
func RateLimited(c *gin.Context, message string, retryAfter time.Duration) {
	if retryAfter > 0 {
		c.Header("Retry-After", strconv.FormatInt(int64(math.Ceil(retryAfter.Seconds())), 10))
	}
	Error(c, http.StatusTooManyRequests, CodeRateLimited, message)
}

// RateLimitedWithPolicy 返回 429，并写入登录限流策略相关的 RateLimit-* 响应头。
func RateLimitedWithPolicy(c *gin.Context, message string, retryAfter time.Duration, limit int, remaining int) {
	if limit > 0 {
		c.Header("RateLimit-Limit", strconv.Itoa(limit))
	}
	if remaining >= 0 {
		c.Header("RateLimit-Remaining", strconv.Itoa(remaining))
	}
	if retryAfter > 0 {
		c.Header("RateLimit-Reset", strconv.FormatInt(int64(math.Ceil(retryAfter.Seconds())), 10))
	}
	RateLimited(c, message, retryAfter)
}

// PayloadTooLarge 返回请求体超过上限的 413 错误。
func PayloadTooLarge(c *gin.Context, message string) {
	Error(c, http.StatusRequestEntityTooLarge, CodePayloadTooLarge, message)
}

// UnsupportedMediaType 返回请求 Content-Type 不被当前接口支持的 415 错误。
func UnsupportedMediaType(c *gin.Context, message string) {
	Error(c, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, message)
}

// NotAcceptable 返回请求 Accept 无法和接口响应类型协商的 406 错误。
func NotAcceptable(c *gin.Context, message string) {
	Error(c, http.StatusNotAcceptable, CodeNotAcceptable, message)
}

// GatewayTimeout 返回服务端请求预算耗尽的 504 错误。
func GatewayTimeout(c *gin.Context, message string) {
	Error(c, http.StatusGatewayTimeout, CodeRequestTimeout, message)
}

// ClientClosedRequest 返回非标准 499，用于标记客户端取消请求。
func ClientClosedRequest(c *gin.Context, message string) {
	Error(c, StatusClientClosedRequest, CodeClientClosedRequest, message)
}

// InternalError 把常见 context 错误映射成稳定状态码，发布模式下隐藏内部错误细节。
func InternalError(c *gin.Context, err error) {
	if errors.Is(err, context.DeadlineExceeded) {
		GatewayTimeout(c, "request timed out")
		return
	}
	if errors.Is(err, context.Canceled) {
		ClientClosedRequest(c, "client closed request")
		return
	}

	message := "internal server error"
	if gin.Mode() != gin.ReleaseMode && err != nil {
		message = err.Error()
	}
	Error(c, http.StatusInternalServerError, CodeInternal, message)
}

// BindJSONBody 使用严格 JSON 解码：禁止未知字段、禁止多个 JSON 值，并执行 Gin binding 校验。
func BindJSONBody(c *gin.Context, dst any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON value")
	}
	if binding.Validator != nil {
		if err := binding.Validator.ValidateStruct(dst); err != nil {
			return err
		}
	}
	return nil
}

// BindQuery 统一入口绑定 query 参数，handler 只需要决定失败时返回的业务提示。
func BindQuery(c *gin.Context, dst any) error {
	return c.ShouldBindQuery(dst)
}

// SetPaginationHeaders 写入总数和 RFC 8288 Link 分页头；非法分页参数会被忽略。
func SetPaginationHeaders(c *gin.Context, total int64, page, pageSize int) {
	if total < 0 || page < 1 || pageSize < 1 {
		return
	}
	c.Header("X-Total-Count", strconv.FormatInt(total, 10))
	if link := paginationLinkHeader(c.Request.URL, total, page, pageSize); link != "" {
		c.Header("Link", link)
	}
}

// SetResourceLocation 为创建成功或已排队的资源写入相对 Location 头。
func SetResourceLocation(c *gin.Context, id uint) {
	if id == 0 {
		return
	}
	c.Header("Location", strings.TrimRight(c.Request.URL.Path, "/")+"/"+strconv.FormatUint(uint64(id), 10))
}

// SetCreatedLocation 为创建成功的资源写入相对 Location 头。
func SetCreatedLocation(c *gin.Context, id uint) {
	SetResourceLocation(c, id)
}

func paginationLinkHeader(requestURL *url.URL, total int64, page, pageSize int) string {
	totalPages := ((total - 1) / int64(pageSize)) + 1
	if requestURL == nil || totalPages <= 1 {
		return ""
	}

	links := make([]string, 0, 4)
	add := func(targetPage int64, rel string) {
		if targetPage < 1 || targetPage > totalPages {
			return
		}
		u := *requestURL
		query := u.Query()
		removeSensitiveQueryParams(query)
		query.Set("page", strconv.FormatInt(targetPage, 10))
		query.Set("pageSize", strconv.Itoa(pageSize))
		u.RawQuery = query.Encode()
		links = append(links, fmt.Sprintf("<%s>; rel=\"%s\"", u.RequestURI(), rel))
	}

	currentPage := int64(page)
	if currentPage > 1 {
		add(1, "first")
		add(currentPage-1, "prev")
	}
	if currentPage < totalPages {
		add(currentPage+1, "next")
		add(totalPages, "last")
	}

	return strings.Join(links, ", ")
}

func removeSensitiveQueryParams(query url.Values) {
	for key := range query {
		if sensitiveQueryParam(key) {
			delete(query, key)
		}
	}
}

func sensitiveQueryParam(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "access_token", "auth_token", "authorization", "jwt", "password", "secret", "token":
		return true
	default:
		return false
	}
}

func codeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return CodeBadRequest
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusMethodNotAllowed:
		return CodeMethodNotAllowed
	case http.StatusTooManyRequests:
		return CodeRateLimited
	case http.StatusRequestEntityTooLarge:
		return CodePayloadTooLarge
	case http.StatusUnsupportedMediaType:
		return CodeUnsupportedMediaType
	case http.StatusNotAcceptable:
		return CodeNotAcceptable
	case http.StatusGatewayTimeout:
		return CodeRequestTimeout
	case StatusClientClosedRequest:
		return CodeClientClosedRequest
	default:
		return CodeInternal
	}
}

func requestIDFromContext(c *gin.Context) string {
	value, ok := c.Get(requestIDContextKey)
	if !ok {
		return ""
	}
	requestID, ok := value.(string)
	if !ok || !validRequestID(requestID) {
		return ""
	}
	return requestID
}

func validRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if r < 33 || r > 126 {
			return false
		}
	}
	return true
}

type responseLogError struct {
	status int
	code   string
}

func (err responseLogError) Error() string {
	return fmt.Sprintf("status=%d code=%s", err.status, err.code)
}
