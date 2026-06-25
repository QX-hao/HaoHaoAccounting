package middleware

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/gin-gonic/gin"
)

// BodyLimit 统一限制请求体大小：已知 Content-Length 超限时直接拒绝，流式请求交给 MaxBytesReader 在读取时拦截。
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			httputil.PayloadTooLarge(c, bodyLimitMessage(maxBytes))
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// HandleBodyReadError 把 MaxBytesReader 返回的超限错误映射成统一的 413 JSON 响应。
func HandleBodyReadError(c *gin.Context, err error) bool {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		httputil.PayloadTooLarge(c, bodyLimitMessage(maxBytesErr.Limit))
		c.Abort()
		return true
	}
	return false
}

func bodyLimitMessage(maxBytes int64) string {
	return fmt.Sprintf("request body too large: max %d bytes", maxBytes)
}
