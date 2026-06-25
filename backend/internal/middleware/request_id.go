package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	RequestIDHeader     = "X-Request-ID"
	RequestIDContextKey = "request_id"
)

type requestIDStdContextKey struct{}

// RequestID 统一生成或复用请求相关 id，并把同一个值写入响应头、Gin context 和标准 context。
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(RequestIDHeader))
		if !ValidRequestID(requestID) {
			requestID = newRequestID()
		}

		c.Set(RequestIDContextKey, requestID)
		ctx := context.WithValue(c.Request.Context(), requestIDStdContextKey{}, requestID)
		c.Request = c.Request.WithContext(ctx)
		c.Request.Header.Set(RequestIDHeader, requestID)
		c.Header(RequestIDHeader, requestID)
		c.Next()
	}
}

// RequestIDFromContext 从 Gin context 读取当前请求 id，主要供响应构造和日志字段复用。
func RequestIDFromContext(c *gin.Context) string {
	value, ok := c.Get(RequestIDContextKey)
	if !ok {
		return ""
	}
	requestID, _ := value.(string)
	return requestID
}

// RequestIDFromStdContext 从标准 context.Context 读取请求 id，方便服务层脱离 Gin 也能关联日志。
func RequestIDFromStdContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requestIDStdContextKey{}).(string)
	return requestID
}

// ValidRequestID 只接受 1 到 128 字节的可见 ASCII，避免空白、换行或超长值污染日志。
func ValidRequestID(value string) bool {
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

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "request-id-unavailable"
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:])
}
