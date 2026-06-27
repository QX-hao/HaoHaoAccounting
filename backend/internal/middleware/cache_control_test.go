package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNoStore(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(NoStore())
	router.GET("/api/v1/me", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))

	if got := resp.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := resp.Header().Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma = %q", got)
	}
	if got := resp.Header().Get("Expires"); got != "0" {
		t.Fatalf("Expires = %q", got)
	}
}

func TestNoCache(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(NoCache())
	router.GET("/metrics", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	assertNoCacheHeaders(t, resp.Header())
}

func TestNoStoreAPIOnlyAppliesToAPIPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(NoStoreAPI("/api/v1"))
	router.GET("/api/v1/me", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/api/v10/me", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/livez", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	for _, path := range []string{"/api/v1", "/api/v1/me"} {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
		if got := resp.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("%s Cache-Control = %q", path, got)
		}
	}
	for _, path := range []string{"/api/v10/me", "/livez"} {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
		if got := resp.Header().Get("Cache-Control"); got != "" {
			t.Fatalf("%s Cache-Control = %q, want empty", path, got)
		}
	}
}

func TestNoStoreAPITrimsConfiguredPrefixTrailingSlash(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(NoStoreAPI("/api/v1/"))
	router.GET("/api/v1/me", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))

	if got := resp.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

func TestNoStoreAPIWithEmptyOrRootPrefixIsNoop(t *testing.T) {
	for _, prefix := range []string{"", "/"} {
		t.Run(prefix, func(t *testing.T) {
			gin.SetMode(gin.TestMode)

			router := gin.New()
			router.Use(NoStoreAPI(prefix))
			router.GET("/livez", func(c *gin.Context) {
				c.Status(http.StatusNoContent)
			})

			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/livez", nil))

			if got := resp.Header().Get("Cache-Control"); got != "" {
				t.Fatalf("Cache-Control = %q, want empty", got)
			}
			if got := resp.Header().Get("Pragma"); got != "" {
				t.Fatalf("Pragma = %q, want empty", got)
			}
			if got := resp.Header().Get("Expires"); got != "" {
				t.Fatalf("Expires = %q, want empty", got)
			}
		})
	}
}

func TestSetNoStore(t *testing.T) {
	headers := http.Header{}
	SetNoStore(headers)

	if got := headers.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := headers.Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma = %q", got)
	}
	if got := headers.Get("Expires"); got != "0" {
		t.Fatalf("Expires = %q", got)
	}
}

func TestSetNoCache(t *testing.T) {
	headers := http.Header{}
	SetNoCache(headers)

	assertNoCacheHeaders(t, headers)
}

func assertNoCacheHeaders(t *testing.T, headers http.Header) {
	t.Helper()

	if got := headers.Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := headers.Get("Pragma"); got != "no-cache" {
		t.Fatalf("Pragma = %q", got)
	}
	if got := headers.Get("Expires"); got != "0" {
		t.Fatalf("Expires = %q", got)
	}
}
