package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/gin-gonic/gin"
)

func TestRequestTimeoutAddsDeadlineAndPreservesRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID(), RequestTimeout(10*time.Millisecond))
	router.GET("/slow", func(c *gin.Context) {
		ctx := c.Request.Context()
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("expected request context deadline")
		}
		if got := RequestIDFromStdContext(ctx); got != "request-123" {
			t.Fatalf("standard context request id = %q", got)
		}

		select {
		case <-ctx.Done():
			c.String(http.StatusOK, ctx.Err().Error())
		case <-time.After(200 * time.Millisecond):
			t.Fatal("request context was not canceled before test timeout")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	req.Header.Set(RequestIDHeader, "request-123")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Body.String(); got != "context deadline exceeded" {
		t.Fatalf("body = %q", got)
	}
	if got := resp.Header().Get(RequestIDHeader); got != "request-123" {
		t.Fatalf("request id header = %q", got)
	}
}

func TestRequestTimeoutWritesGatewayTimeoutWhenHandlerLeavesResponseEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID(), RequestTimeout(10*time.Millisecond))
	router.GET("/slow", func(c *gin.Context) {
		<-c.Request.Context().Done()
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	req.Header.Set(RequestIDHeader, "request-timeout")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get(RequestIDHeader); got != "request-timeout" {
		t.Fatalf("request id header = %q", got)
	}
	var body httputil.ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != httputil.CodeRequestTimeout || body.RequestID != "request-timeout" {
		t.Fatalf("body = %#v", body)
	}
}

func TestRequestTimeoutDisabledDoesNotSetDeadline(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestTimeout(0))
	router.GET("/ping", func(c *gin.Context) {
		if _, ok := c.Request.Context().Deadline(); ok {
			t.Fatal("expected no request context deadline")
		}
		c.Status(http.StatusNoContent)
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}
