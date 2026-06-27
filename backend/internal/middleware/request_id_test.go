package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestRequestIDMiddlewareIsIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID(), RequestID())
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

func TestNewRequestIDProducesSafeHeaderValue(t *testing.T) {
	requestID := newRequestID()

	if !ValidRequestID(requestID) {
		t.Fatalf("generated request id %q is not a valid request id", requestID)
	}
	if len(requestID) != 22 {
		t.Fatalf("generated request id length = %d, want 22", len(requestID))
	}
	if strings.ContainsAny(requestID, "+/=") {
		t.Fatalf("generated request id should use raw URL-safe base64: %q", requestID)
	}
}

func TestNewRequestIDFallbackStaysUniqueWhenEntropyFails(t *testing.T) {
	withRequestIDEntropyReader(t, failingReader{})

	first := newRequestID()
	second := newRequestID()

	if first == second {
		t.Fatalf("fallback request ids should be unique, got %q twice", first)
	}
	for _, requestID := range []string{first, second} {
		if !ValidRequestID(requestID) {
			t.Fatalf("fallback request id %q is not valid", requestID)
		}
		if !strings.HasPrefix(requestID, "request-id-fallback-") {
			t.Fatalf("fallback request id = %q", requestID)
		}
	}
}

func TestRequestIDFallbackKeepsMiddlewareCorrelationWhenEntropyFails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withRequestIDEntropyReader(t, failingReader{})

	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"requestId":    RequestIDFromContext(c),
			"stdRequestId": RequestIDFromStdContext(c.Request.Context()),
			"header":       c.GetHeader(RequestIDHeader),
		})
	})

	responses := make([]*httptest.ResponseRecorder, 0, 2)
	for range 2 {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ping", nil))
		responses = append(responses, resp)
	}

	firstRequestID := responses[0].Header().Get(RequestIDHeader)
	secondRequestID := responses[1].Header().Get(RequestIDHeader)
	if firstRequestID == secondRequestID {
		t.Fatalf("fallback request ids should be unique across middleware requests, got %q twice", firstRequestID)
	}
	for _, resp := range responses {
		requestID := resp.Header().Get(RequestIDHeader)
		wantBody := `{"header":"` + requestID + `","requestId":"` + requestID + `","stdRequestId":"` + requestID + `"}`
		if resp.Body.String() != wantBody {
			t.Fatalf("body = %q, want %q", resp.Body.String(), wantBody)
		}
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

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("entropy unavailable")
}

func withRequestIDEntropyReader(t *testing.T, reader io.Reader) {
	t.Helper()

	oldReader := requestIDEntropyReader
	oldSequence := requestIDFallbackSequence.Swap(0)
	requestIDEntropyReader = reader
	t.Cleanup(func() {
		requestIDEntropyReader = oldReader
		requestIDFallbackSequence.Store(oldSequence)
	})
}

func TestRequestIDReplacesDuplicateCallerHeadersEverywhere(t *testing.T) {
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
	req.Header.Add(RequestIDHeader, "client-request-1")
	req.Header.Add(RequestIDHeader, "client-request-2")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	requestID := resp.Header().Get(RequestIDHeader)
	if requestID == "" || requestID == "client-request-1" || requestID == "client-request-2" {
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

func TestValidRequestIDBoundsLength(t *testing.T) {
	if !ValidRequestID(strings.Repeat("a", 128)) {
		t.Fatal("128 byte request id should be valid")
	}
	if ValidRequestID(strings.Repeat("a", 129)) {
		t.Fatal("129 byte request id should be invalid")
	}
}

func TestRequestIDFromContextRejectsUnsafeValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name string
		set  func(*gin.Context)
	}{
		{name: "missing"},
		{name: "wrong type", set: func(c *gin.Context) { c.Set(RequestIDContextKey, 42) }},
		{name: "empty", set: func(c *gin.Context) { c.Set(RequestIDContextKey, "") }},
		{name: "control character", set: func(c *gin.Context) { c.Set(RequestIDContextKey, "bad\nid") }},
		{name: "whitespace", set: func(c *gin.Context) { c.Set(RequestIDContextKey, "bad id") }},
		{name: "too long", set: func(c *gin.Context) { c.Set(RequestIDContextKey, strings.Repeat("a", 129)) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tc.set != nil {
				tc.set(c)
			}

			if got := RequestIDFromContext(c); got != "" {
				t.Fatalf("RequestIDFromContext = %q, want empty", got)
			}
		})
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

func TestRequestIDFromStdContextRejectsUnsafeValues(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
	}{
		{name: "wrong type", value: 42},
		{name: "empty", value: ""},
		{name: "control character", value: "bad\nid"},
		{name: "whitespace", value: "bad id"},
		{name: "too long", value: strings.Repeat("a", 129)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), requestIDStdContextKey{}, tc.value)

			if got := RequestIDFromStdContext(ctx); got != "" {
				t.Fatalf("RequestIDFromStdContext = %q, want empty", got)
			}
		})
	}
}
