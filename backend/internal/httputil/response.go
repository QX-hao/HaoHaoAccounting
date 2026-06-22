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
	RequestID string `json:"requestId"`
}

func Error(c *gin.Context, status int, code string, message string) {
	if c.Writer.Written() {
		return
	}
	if code == "" {
		code = codeForStatus(status)
	}
	_ = c.Error(responseLogError{status: status, code: code})
	c.JSON(status, ErrorResponse{
		Error:     message,
		Code:      code,
		RequestID: requestIDFromContext(c),
	})
}

func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, CodeBadRequest, message)
}

func InvalidRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, CodeInvalidRequest, message)
}

func Unauthorized(c *gin.Context, message string) {
	c.Header("WWW-Authenticate", bearerChallenge)
	Error(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

func InvalidToken(c *gin.Context, message string) {
	c.Header("WWW-Authenticate", invalidBearerTokenChallenge)
	Error(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, CodeForbidden, message)
}

func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, CodeNotFound, message)
}

func MethodNotAllowed(c *gin.Context, message string) {
	Error(c, http.StatusMethodNotAllowed, CodeMethodNotAllowed, message)
}

func RateLimited(c *gin.Context, message string, retryAfter time.Duration) {
	if retryAfter > 0 {
		c.Header("Retry-After", strconv.FormatInt(int64(math.Ceil(retryAfter.Seconds())), 10))
	}
	Error(c, http.StatusTooManyRequests, CodeRateLimited, message)
}

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

func PayloadTooLarge(c *gin.Context, message string) {
	Error(c, http.StatusRequestEntityTooLarge, CodePayloadTooLarge, message)
}

func UnsupportedMediaType(c *gin.Context, message string) {
	Error(c, http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, message)
}

func NotAcceptable(c *gin.Context, message string) {
	Error(c, http.StatusNotAcceptable, CodeNotAcceptable, message)
}

func GatewayTimeout(c *gin.Context, message string) {
	Error(c, http.StatusGatewayTimeout, CodeRequestTimeout, message)
}

func ClientClosedRequest(c *gin.Context, message string) {
	Error(c, StatusClientClosedRequest, CodeClientClosedRequest, message)
}

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

func SetPaginationHeaders(c *gin.Context, total int64, page, pageSize int) {
	if total < 0 || page < 1 || pageSize < 1 {
		return
	}
	c.Header("X-Total-Count", strconv.FormatInt(total, 10))
	if link := paginationLinkHeader(c.Request.URL, total, page, pageSize); link != "" {
		c.Header("Link", link)
	}
}

func SetCreatedLocation(c *gin.Context, id uint) {
	if id == 0 {
		return
	}
	c.Header("Location", strings.TrimRight(c.Request.URL.Path, "/")+"/"+strconv.FormatUint(uint64(id), 10))
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
	requestID, _ := value.(string)
	return requestID
}

type responseLogError struct {
	status int
	code   string
}

func (err responseLogError) Error() string {
	return fmt.Sprintf("status=%d code=%s", err.status, err.code)
}
