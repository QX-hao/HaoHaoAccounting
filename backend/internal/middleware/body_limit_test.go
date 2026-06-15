package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/gin-gonic/gin"
)

func TestBodyLimitRejectsOversizedContentLength(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID(), BodyLimit(4))
	router.POST("/echo", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("12345"))
	req.Header.Set(RequestIDHeader, "request-123")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if got := resp.Header().Get(RequestIDHeader); got != "request-123" {
		t.Fatalf("request id header = %q", got)
	}

	var body httputil.ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != httputil.CodePayloadTooLarge {
		t.Fatalf("code = %q", body.Code)
	}
	if body.RequestID != "request-123" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
}

func TestBodyLimitAllowsBodyWithinLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(BodyLimit(5))
	router.POST("/echo", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("12345")))

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestHandleBodyReadErrorWritesPayloadTooLarge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(RequestID(), BodyLimit(4))
	router.POST("/json", func(c *gin.Context) {
		var body map[string]any
		if err := c.ShouldBindJSON(&body); err != nil {
			if HandleBodyReadError(c, err) {
				return
			}
			httputil.InvalidRequest(c, "invalid request body")
			return
		}
		c.JSON(http.StatusOK, body)
	})

	req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader(`{"x":1}`))
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(RequestIDHeader, "request-456")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var body httputil.ErrorResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Code != httputil.CodePayloadTooLarge {
		t.Fatalf("code = %q", body.Code)
	}
	if body.RequestID != "request-456" {
		t.Fatalf("requestId = %q", body.RequestID)
	}
}

func TestHandleBodyReadErrorAbortsContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	calledNext := false
	router := gin.New()
	router.Use(RequestID(), BodyLimit(4))
	router.POST("/json", func(c *gin.Context) {
		var body map[string]any
		if err := c.ShouldBindJSON(&body); err != nil {
			if HandleBodyReadError(c, err) {
				c.Next()
				return
			}
			httputil.InvalidRequest(c, "invalid request body")
			return
		}
		c.JSON(http.StatusOK, body)
	}, func(c *gin.Context) {
		calledNext = true
		c.Status(http.StatusTeapot)
	})

	req := httptest.NewRequest(http.MethodPost, "/json", strings.NewReader(`{"x":1}`))
	req.ContentLength = -1
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if calledNext {
		t.Fatal("expected HandleBodyReadError to abort pending handlers")
	}
}
