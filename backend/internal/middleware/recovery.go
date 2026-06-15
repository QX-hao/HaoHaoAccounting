package middleware

import (
	"fmt"
	"io"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return recoveryWithWriter(gin.DefaultErrorWriter)
}

func recoveryWithWriter(out io.Writer) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(out, func(c *gin.Context, recovered any) {
		logRecoveredPanic(out, c, recovered)
		if c.Writer.Written() {
			c.Abort()
			return
		}
		httputil.InternalError(c, fmt.Errorf("panic recovered: %v", recovered))
		c.Abort()
	})
}

func logRecoveredPanic(out io.Writer, c *gin.Context, recovered any) {
	if out == nil {
		return
	}
	fmt.Fprintf(
		out,
		"panic_recovered method=%q path=%q client_ip=%q request_id=%q panic_type=%q panic_value=%q\n",
		c.Request.Method,
		c.Request.URL.Path,
		c.ClientIP(),
		RequestIDFromContext(c),
		fmt.Sprintf("%T", recovered),
		fmt.Sprint(recovered),
	)
}
