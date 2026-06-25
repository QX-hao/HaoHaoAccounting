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

// SecurityHeaders 写入 API 默认浏览器安全头；HSTS 和 COEP 这类部署敏感头通过配置显式开启。
func SecurityHeaders(configs ...SecurityHeadersConfig) gin.HandlerFunc {
	cfg := SecurityHeadersConfig{}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	hsts := strictTransportSecurityValue(cfg)
	coep := normalizedCrossOriginEmbedderPolicy(cfg.CrossOriginEmbedderPolicy)
	return func(c *gin.Context) {
		headers := c.Writer.Header()
		for key, value := range defaultSecurityHeaders() {
			headers.Set(key, value)
		}
		if coep != "" {
			headers.Set("Cross-Origin-Embedder-Policy", coep)
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

func normalizedCrossOriginEmbedderPolicy(value string) string {
	// COEP 只能写入浏览器识别的固定值，避免调用方绕过配置校验后把任意字符串写进响应头。
	switch clean := strings.ToLower(strings.TrimSpace(value)); clean {
	case "require-corp", "credentialless", "unsafe-none":
		return clean
	default:
		return ""
	}
}
