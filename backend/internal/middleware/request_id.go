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

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(RequestIDHeader))
		if !validRequestID(requestID) {
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

func RequestIDFromContext(c *gin.Context) string {
	value, ok := c.Get(RequestIDContextKey)
	if !ok {
		return ""
	}
	requestID, _ := value.(string)
	return requestID
}

func RequestIDFromStdContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requestIDStdContextKey{}).(string)
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

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "request-id-unavailable"
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:])
}
