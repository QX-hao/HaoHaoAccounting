package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/gin-gonic/gin"
)

// RequestTimeout adds a deadline to the request context without writing a timeout response.
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
		if ctx.Err() == context.DeadlineExceeded && !c.Writer.Written() {
			httputil.GatewayTimeout(c, http.StatusText(http.StatusGatewayTimeout))
			c.Abort()
		}
	}
}
