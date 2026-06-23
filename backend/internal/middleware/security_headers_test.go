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
	for key, want := range defaultSecurityHeaders() {
		if got := headers.Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	if got := headers.Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("Strict-Transport-Security = %q, want empty by default", got)
	}
	if got := headers.Get("Cross-Origin-Embedder-Policy"); got != "" {
		t.Fatalf("Cross-Origin-Embedder-Policy = %q, want empty by default", got)
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

func TestSecurityHeadersSetsConfiguredCrossOriginEmbedderPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders(SecurityHeadersConfig{
		CrossOriginEmbedderPolicy: "require-corp",
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
