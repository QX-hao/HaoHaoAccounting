package middleware

import (
	"context"
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
	if body.Error != "request timed out" {
		t.Fatalf("error = %q", body.Error)
	}
}

func TestRequestTimeoutMapsCanceledParentContextToClientClosedRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID(), RequestTimeout(time.Second))
	router.GET("/slow", func(c *gin.Context) {
		<-c.Request.Context().Done()
	})

	parent, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/slow", nil).WithContext(parent)
	req.Header.Set(RequestIDHeader, "request-canceled")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != httputil.StatusClientClosedRequest {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get(RequestIDHeader); got != "request-canceled" {
		t.Fatalf("request id header = %q", got)
	}
	var body httputil.ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != httputil.CodeClientClosedRequest || body.RequestID != "request-canceled" {
		t.Fatalf("body = %#v", body)
	}
	if body.Error != "client closed request" {
		t.Fatalf("error = %q", body.Error)
	}
}

func TestRequestTimeoutDoesNotOverwriteWrittenResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestTimeout(10 * time.Millisecond))
	router.GET("/stream", func(c *gin.Context) {
		c.String(http.StatusAccepted, "accepted")
		<-c.Request.Context().Done()
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/stream", nil))

	if resp.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Body.String(); got != "accepted" {
		t.Fatalf("body = %q", got)
	}
}

func TestRequestTimeoutDisabledDoesNotSetDeadline(t *testing.T) {
	for _, timeout := range []time.Duration{0, -time.Second} {
		t.Run(timeout.String(), func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			router := gin.New()
			router.Use(RequestTimeout(timeout))
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
		})
	}
}
