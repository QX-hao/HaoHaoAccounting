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
	"syscall"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/stringutil"
	"github.com/gin-gonic/gin"
)

const maxLoggedPanicValueLength = 512

// Recovery 捕获 handler panic，记录受控日志，并在响应尚未写出时返回统一的 500 JSON。
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
	panicValue := logPanicValue(recovered)
	if isBrokenPipe(recovered) {
		fmt.Fprintf(
			out,
			"panic_recovered method=%q path=%q client_ip=%q request_id=%q panic_type=%q panic_value=%q\n",
			c.Request.Method,
			c.Request.URL.Path,
			c.ClientIP(),
			RequestIDFromContext(c),
			fmt.Sprintf("%T", recovered),
			panicValue,
		)
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
		panicValue,
		string(debug.Stack()),
	)
}

func logPanicValue(recovered any) string {
	// panic 值可能是很长的字符串或错误对象，日志里保留前缀即可定位问题。
	return stringutil.TruncateRunes(fmt.Sprint(recovered), maxLoggedPanicValueLength)
}

func isBrokenPipe(recovered any) bool {
	err, ok := recovered.(error)
	if !ok {
		return false
	}
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, http.ErrAbortHandler) {
		return true
	}
	netErr, ok := err.(*net.OpError)
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
