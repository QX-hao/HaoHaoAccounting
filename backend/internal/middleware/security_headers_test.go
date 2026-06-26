package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersSetsDefaultHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ping", nil))

	headers := recorder.Result().Header
	seen := make(map[string]struct{}, len(defaultSecurityHeaders))
	for _, header := range defaultSecurityHeaders {
		if _, ok := seen[header.Key]; ok {
			t.Fatalf("default security header %q is duplicated", header.Key)
		}
		seen[header.Key] = struct{}{}
		if got := headers.Get(header.Key); got != header.Value {
			t.Fatalf("%s = %q, want %q", header.Key, got, header.Value)
		}
	}
	if got := headers.Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("Strict-Transport-Security = %q, want empty by default", got)
	}
	if got := headers.Get("Cross-Origin-Embedder-Policy"); got != "" {
		t.Fatalf("Cross-Origin-Embedder-Policy = %q, want empty by default", got)
	}
}

func TestSecurityHeadersArePresentOnWrittenResponses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if got := recorder.Result().Header.Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY", got)
	}
	if got := recorder.Result().Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

func TestSecurityHeadersSetsConfiguredHSTS(t *testing.T) {
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

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ping", nil))

	want := "max-age=31536000; includeSubDomains; preload"
	if got := recorder.Result().Header.Get("Strict-Transport-Security"); got != want {
		t.Fatalf("Strict-Transport-Security = %q, want %q", got, want)
	}
}

func TestSecurityHeadersHSTSPreloadIncludesSubDomains(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders(SecurityHeadersConfig{
		HSTSMaxAgeSeconds: 31536000,
		HSTSPreload:       true,
	}))
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ping", nil))

	want := "max-age=31536000; includeSubDomains; preload"
	if got := recorder.Result().Header.Get("Strict-Transport-Security"); got != want {
		t.Fatalf("Strict-Transport-Security = %q, want %q", got, want)
	}
}

func TestSecurityHeadersSetsConfiguredCrossOriginEmbedderPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders(SecurityHeadersConfig{
		CrossOriginEmbedderPolicy: " Require-Corp ",
	}))
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if got := recorder.Result().Header.Get("Cross-Origin-Embedder-Policy"); got != "require-corp" {
		t.Fatalf("Cross-Origin-Embedder-Policy = %q, want require-corp", got)
	}
}

func TestSecurityHeadersRejectsUnknownCrossOriginEmbedderPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders(SecurityHeadersConfig{
		CrossOriginEmbedderPolicy: "same-origin",
	}))
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if got := recorder.Result().Header.Get("Cross-Origin-Embedder-Policy"); got != "" {
		t.Fatalf("Cross-Origin-Embedder-Policy = %q, want empty for unknown policy", got)
	}
}
