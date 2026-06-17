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
	for key, want := range map[string]string{
		"Cross-Origin-Opener-Policy":        "same-origin",
		"Cross-Origin-Resource-Policy":      "same-origin",
		"Content-Security-Policy":           "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'",
		"Origin-Agent-Cluster":              "?1",
		"Referrer-Policy":                   "no-referrer",
		"Permissions-Policy":                "camera=(), geolocation=(), microphone=(), payment=()",
		"X-Content-Type-Options":            "nosniff",
		"X-DNS-Prefetch-Control":            "off",
		"X-Download-Options":                "noopen",
		"X-Frame-Options":                   "DENY",
		"X-Permitted-Cross-Domain-Policies": "none",
		"X-XSS-Protection":                  "0",
	} {
		if got := headers.Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
	if got := headers.Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("Strict-Transport-Security = %q, want empty by default", got)
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
