package middleware

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return recoveryWithWriter(gin.DefaultErrorWriter)
}

func recoveryWithWriter(out io.Writer) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logRecoveredPanic(out, c, recovered)
				if isBrokenPipe(recovered) {
					if err, ok := recovered.(error); ok {
						_ = c.Error(err)
					}
					c.Abort()
					return
				}
				if c.Writer.Written() {
					c.Abort()
					return
				}
				httputil.Error(c, http.StatusInternalServerError, httputil.CodeInternal, "internal server error")
				c.Abort()
			}
		}()
		c.Next()
	}
}

func logRecoveredPanic(out io.Writer, c *gin.Context, recovered any) {
	if out == nil {
		return
	}
	fmt.Fprintf(
		out,
		"panic_recovered method=%q path=%q client_ip=%q request_id=%q panic_type=%q panic_value=%q stack=%q\n",
		c.Request.Method,
		c.Request.URL.Path,
		c.ClientIP(),
		RequestIDFromContext(c),
		fmt.Sprintf("%T", recovered),
		fmt.Sprint(recovered),
		string(debug.Stack()),
	)
}

func isBrokenPipe(recovered any) bool {
	netErr, ok := recovered.(*net.OpError)
	if !ok {
		return false
	}
	var syscallErr *os.SyscallError
	if !errors.As(netErr, &syscallErr) {
		return false
	}
	message := strings.ToLower(syscallErr.Error())
	return strings.Contains(message, "broken pipe") || strings.Contains(message, "connection reset by peer")
}
