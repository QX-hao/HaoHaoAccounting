package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/gin-gonic/gin"
)

func TestRecoveryReturnsStructuredInternalError(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(gin.TestMode) })

	var panicLog bytes.Buffer
	router := gin.New()
	router.Use(RequestID(), recoveryWithWriter(&panicLog))
	router.GET("/panic", func(*gin.Context) {
		panic("sensitive panic details")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set(RequestIDHeader, "request-123")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get(RequestIDHeader); got != "request-123" {
		t.Fatalf("request id header = %q", got)
	}

	var body httputil.ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != httputil.CodeInternal {
		t.Fatalf("code = %q", body.Code)
	}
	if body.Error != "internal server error" {
		t.Fatalf("error = %q", body.Error)
	}
	if body.RequestID != "request-123" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
	logOutput := panicLog.String()
	if strings.Contains(logOutput, "[Recovery]") {
		t.Fatalf("panic log contains duplicate Gin recovery log: %q", logOutput)
	}
	for _, want := range []string{
		`panic_recovered`,
		`method="GET"`,
		`path="/panic"`,
		`request_id="request-123"`,
		`panic_type="string"`,
		`panic_value="sensitive panic details"`,
		`stack="goroutine `,
		`TestRecoveryReturnsStructuredInternalError`,
	} {
		if !strings.Contains(logOutput, want) {
			t.Fatalf("panic log = %q, missing %s", logOutput, want)
		}
	}
	if strings.Contains(resp.Body.String(), "sensitive panic details") {
		t.Fatalf("response leaked panic details: %s", resp.Body.String())
	}
}

func TestRecoveryPanicLogOmitsQueryString(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(gin.TestMode) })

	var panicLog bytes.Buffer
	router := gin.New()
	router.Use(RequestID(), recoveryWithWriter(&panicLog))
	router.GET("/panic", func(*gin.Context) {
		panic("boom")
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/panic?token=secret&password=hidden", nil))

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	logOutput := panicLog.String()
	if !strings.Contains(logOutput, `path="/panic"`) {
		t.Fatalf("panic log missing sanitized path: %q", logOutput)
	}
	for _, leaked := range []string{"/panic?", "token=secret", "password=hidden"} {
		if strings.Contains(logOutput, leaked) {
			t.Fatalf("panic log leaked query data %q: %q", leaked, logOutput)
		}
	}
}

func TestRecoveryDoesNotLeakPanicDetailsOutsideRelease(t *testing.T) {
	gin.SetMode(gin.DebugMode)
	t.Cleanup(func() { gin.SetMode(gin.TestMode) })

	router := gin.New()
	router.Use(RequestID(), recoveryWithWriter(nil))
	router.GET("/panic", func(*gin.Context) {
		panic("debug panic details")
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var body httputil.ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Error != "internal server error" {
		t.Fatalf("error = %q", body.Error)
	}
	if strings.Contains(resp.Body.String(), "debug panic details") {
		t.Fatalf("response leaked panic details: %s", resp.Body.String())
	}
}

func TestRecoveryDoesNotWriteErrorForBrokenPipe(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(gin.TestMode) })

	var panicLog bytes.Buffer
	router := gin.New()
	router.Use(RequestID(), recoveryWithWriter(&panicLog))
	router.GET("/panic-broken-pipe", func(*gin.Context) {
		panic(&net.OpError{Err: &os.SyscallError{Err: errors.New("broken pipe")}})
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/panic-broken-pipe", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if resp.Body.String() != "" {
		t.Fatalf("body = %q", resp.Body.String())
	}
	logOutput := panicLog.String()
	for _, want := range []string{
		`panic_recovered`,
		`path="/panic-broken-pipe"`,
		`broken pipe`,
	} {
		if !strings.Contains(logOutput, want) {
			t.Fatalf("panic log = %q, missing %s", logOutput, want)
		}
	}
	if strings.Contains(logOutput, `stack="`) {
		t.Fatalf("broken pipe panic log should not include stack: %q", logOutput)
	}
}

func TestRecoveryDoesNotWriteErrorForConnectionAbortErrors(t *testing.T) {
	tests := []struct {
		name      string
		recovered error
		wantLog   string
	}{
		{
			name:      "http abort handler",
			recovered: http.ErrAbortHandler,
			wantLog:   "net/http: abort Handler",
		},
		{
			name:      "wrapped connection reset",
			recovered: &net.OpError{Err: &os.SyscallError{Err: syscall.ECONNRESET}},
			wantLog:   "connection reset by peer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.ReleaseMode)
			t.Cleanup(func() { gin.SetMode(gin.TestMode) })

			var panicLog bytes.Buffer
			router := gin.New()
			router.Use(RequestID(), recoveryWithWriter(&panicLog))
			router.GET("/panic-abort", func(*gin.Context) {
				panic(tt.recovered)
			})

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/panic-abort", nil))

			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
			}
			if resp.Body.String() != "" {
				t.Fatalf("body = %q", resp.Body.String())
			}
			logOutput := panicLog.String()
			for _, want := range []string{
				`panic_recovered`,
				`path="/panic-abort"`,
				tt.wantLog,
			} {
				if !strings.Contains(logOutput, want) {
					t.Fatalf("panic log = %q, missing %s", logOutput, want)
				}
			}
			if strings.Contains(logOutput, `stack="`) {
				t.Fatalf("connection abort panic log should not include stack: %q", logOutput)
			}
		})
	}
}

func TestRecoveryDoesNotWriteErrorAfterResponseStarted(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	t.Cleanup(func() { gin.SetMode(gin.TestMode) })

	var panicLog bytes.Buffer
	router := gin.New()
	router.Use(RequestID(), recoveryWithWriter(&panicLog))
	router.GET("/panic-after-write", func(c *gin.Context) {
		c.String(http.StatusOK, "already written")
		panic("late panic")
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/panic-after-write", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Body.String(); got != "already written" {
		t.Fatalf("body = %q", got)
	}
	logOutput := panicLog.String()
	for _, want := range []string{
		`panic_recovered`,
		`path="/panic-after-write"`,
		`panic_value="late panic"`,
	} {
		if !strings.Contains(logOutput, want) {
			t.Fatalf("panic log = %q, missing %s", logOutput, want)
		}
	}
}
