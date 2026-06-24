package middleware

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type SecurityHeadersConfig struct {
	HSTSMaxAgeSeconds         int
	HSTSIncludeSubDomains     bool
	HSTSPreload               bool
	CrossOriginEmbedderPolicy string
}

func SecurityHeaders(configs ...SecurityHeadersConfig) gin.HandlerFunc {
	cfg := SecurityHeadersConfig{}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	hsts := strictTransportSecurityValue(cfg)
	return func(c *gin.Context) {
		headers := c.Writer.Header()
		for key, value := range defaultSecurityHeaders() {
			headers.Set(key, value)
		}
		if cfg.CrossOriginEmbedderPolicy != "" {
			headers.Set("Cross-Origin-Embedder-Policy", cfg.CrossOriginEmbedderPolicy)
		}
		if hsts != "" {
			headers.Set("Strict-Transport-Security", hsts)
		}
		c.Next()
	}
}

func defaultSecurityHeaders() map[string]string {
	return map[string]string{
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
	}
}

func strictTransportSecurityValue(cfg SecurityHeadersConfig) string {
	if cfg.HSTSMaxAgeSeconds <= 0 {
		return ""
	}
	directives := []string{"max-age=" + strconv.Itoa(cfg.HSTSMaxAgeSeconds)}
	if cfg.HSTSIncludeSubDomains {
		directives = append(directives, "includeSubDomains")
	}
	if cfg.HSTSPreload {
		directives = append(directives, "preload")
	}
	return strings.Join(directives, "; ")
}
