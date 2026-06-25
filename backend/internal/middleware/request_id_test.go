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

func TestRequestIDNormalizesCallerHeaderWhitespace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"requestId": RequestIDFromContext(c),
			"header":    c.GetHeader(RequestIDHeader),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(RequestIDHeader, "  client-request-123  ")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if got := resp.Header().Get(RequestIDHeader); got != "client-request-123" {
		t.Fatalf("response request id = %q, want normalized caller request id", got)
	}
	if got := resp.Body.String(); got != `{"header":"client-request-123","requestId":"client-request-123"}` {
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
	if ValidRequestID("") {
		t.Fatal("empty request id should be invalid")
	}
	if ValidRequestID("bad\nid") {
		t.Fatal("request id with control characters should be invalid")
	}
	if ValidRequestID("bad id") {
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
