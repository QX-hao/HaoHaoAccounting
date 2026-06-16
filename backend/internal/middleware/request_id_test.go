package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestIDPreservesCallerHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"requestId":    RequestIDFromContext(c),
			"stdRequestId": RequestIDFromStdContext(c.Request.Context()),
			"header":       c.GetHeader(RequestIDHeader),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(RequestIDHeader, "client-request-123")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if got := resp.Header().Get(RequestIDHeader); got != "client-request-123" {
		t.Fatalf("response request id = %q, want caller request id", got)
	}
	if got := resp.Body.String(); got != `{"header":"client-request-123","requestId":"client-request-123","stdRequestId":"client-request-123"}` {
		t.Fatalf("body = %s", got)
	}
}

func TestRequestIDGeneratesWhenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"requestId": RequestIDFromContext(c),
			"header":    c.GetHeader(RequestIDHeader),
		})
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ping", nil))

	requestID := resp.Header().Get(RequestIDHeader)
	if requestID == "" {
		t.Fatal("expected generated request id response header")
	}
	wantBody := `{"header":"` + requestID + `","requestId":"` + requestID + `"}`
	if resp.Body.String() != wantBody {
		t.Fatalf("body = %q, want %q", resp.Body.String(), wantBody)
	}
}

func TestRequestIDReplacesInvalidCallerHeaderEverywhere(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"requestId":    RequestIDFromContext(c),
			"stdRequestId": RequestIDFromStdContext(c.Request.Context()),
			"header":       c.GetHeader(RequestIDHeader),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(RequestIDHeader, " bad ")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	requestID := resp.Header().Get(RequestIDHeader)
	if requestID == "" || requestID == " bad " {
		t.Fatalf("response request id = %q", requestID)
	}

	wantBody := `{"header":"` + requestID + `","requestId":"` + requestID + `","stdRequestId":"` + requestID + `"}`
	if resp.Body.String() != wantBody {
		t.Fatalf("body = %q, want %q", resp.Body.String(), wantBody)
	}
}

func TestValidRequestIDRejectsUnsafeValues(t *testing.T) {
	if validRequestID("") {
		t.Fatal("empty request id should be invalid")
	}
	if validRequestID("bad\nid") {
		t.Fatal("request id with control characters should be invalid")
	}
	if validRequestID("  padded  ") {
		t.Fatal("request id with spaces should be invalid")
	}
}

func TestRequestIDFromStdContextReturnsEmptyWhenMissing(t *testing.T) {
	if got := RequestIDFromStdContext(nil); got != "" {
		t.Fatalf("nil context request id = %q", got)
	}
	if got := RequestIDFromStdContext(httptest.NewRequest(http.MethodGet, "/ping", nil).Context()); got != "" {
		t.Fatalf("empty context request id = %q", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if got := resp.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := resp.Header().Get("Cross-Origin-Opener-Policy"); got != "same-origin" {
		t.Fatalf("Cross-Origin-Opener-Policy = %q", got)
	}
	if got := resp.Header().Get("Cross-Origin-Resource-Policy"); got != "same-origin" {
		t.Fatalf("Cross-Origin-Resource-Policy = %q", got)
	}
	if got := resp.Header().Get("Content-Security-Policy"); got != "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'" {
		t.Fatalf("Content-Security-Policy = %q", got)
	}
	if got := resp.Header().Get("Origin-Agent-Cluster"); got != "?1" {
		t.Fatalf("Origin-Agent-Cluster = %q", got)
	}
	if got := resp.Header().Get("X-DNS-Prefetch-Control"); got != "off" {
		t.Fatalf("X-DNS-Prefetch-Control = %q", got)
	}
	if got := resp.Header().Get("X-Download-Options"); got != "noopen" {
		t.Fatalf("X-Download-Options = %q", got)
	}
	if got := resp.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q", got)
	}
	if got := resp.Header().Get("X-Permitted-Cross-Domain-Policies"); got != "none" {
		t.Fatalf("X-Permitted-Cross-Domain-Policies = %q", got)
	}
	if got := resp.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
	if got := resp.Header().Get("Permissions-Policy"); got != "camera=(), geolocation=(), microphone=(), payment=()" {
		t.Fatalf("Permissions-Policy = %q", got)
	}
	if got := resp.Header().Get("X-XSS-Protection"); got != "0" {
		t.Fatalf("X-XSS-Protection = %q", got)
	}
	if got := resp.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("Strict-Transport-Security = %q", got)
	}
}

func TestSecurityHeadersIncludesConfiguredHSTS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(SecurityHeaders(SecurityHeadersConfig{
		HSTSMaxAgeSeconds:     31536000,
		HSTSIncludeSubDomains: true,
		HSTSPreload:           true,
	}))
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if got := resp.Header().Get("Strict-Transport-Security"); got != "max-age=31536000; includeSubDomains; preload" {
		t.Fatalf("Strict-Transport-Security = %q", got)
	}
}
