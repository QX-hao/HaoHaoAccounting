package middleware

import (
	"context"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/gin-gonic/gin"
)

// RequestTimeout 给请求 context 设置截止时间；handler 未写响应时把超时统一转换成结构化 504。
func RequestTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if timeout <= 0 {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
		// handler 如果尊重 request.Context()，这里会在尚未写响应时统一转成结构化 504。
		// 已写出的响应不再覆盖，避免把流式/部分成功响应改坏。
		if ctx.Err() == context.DeadlineExceeded && !c.Writer.Written() {
			httputil.GatewayTimeout(c, "request timed out")
			c.Abort()
		}
	}
}
