package middleware

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type SecurityHeadersConfig struct {
	HSTSMaxAgeSeconds     int
	HSTSIncludeSubDomains bool
	HSTSPreload           bool
}

func SecurityHeaders(configs ...SecurityHeadersConfig) gin.HandlerFunc {
	cfg := SecurityHeadersConfig{}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	hsts := strictTransportSecurityValue(cfg)
	return func(c *gin.Context) {
		headers := c.Writer.Header()
		headers.Set("Cross-Origin-Opener-Policy", "same-origin")
		headers.Set("Cross-Origin-Resource-Policy", "same-origin")
		headers.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		headers.Set("Origin-Agent-Cluster", "?1")
		headers.Set("Referrer-Policy", "no-referrer")
		headers.Set("Permissions-Policy", "camera=(), geolocation=(), microphone=(), payment=()")
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("X-DNS-Prefetch-Control", "off")
		headers.Set("X-Frame-Options", "DENY")
		headers.Set("X-XSS-Protection", "0")
		if hsts != "" {
			headers.Set("Strict-Transport-Security", hsts)
		}
		c.Next()
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
