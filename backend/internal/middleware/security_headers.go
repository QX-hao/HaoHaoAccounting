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

type securityHeader struct {
	Key   string
	Value string
}

var defaultSecurityHeaders = []securityHeader{
	{Key: "Cross-Origin-Opener-Policy", Value: "same-origin"},
	{Key: "Cross-Origin-Resource-Policy", Value: "same-origin"},
	{Key: "Content-Security-Policy", Value: "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"},
	{Key: "Origin-Agent-Cluster", Value: "?1"},
	{Key: "Referrer-Policy", Value: "no-referrer"},
	{Key: "Permissions-Policy", Value: "camera=(), geolocation=(), microphone=(), payment=()"},
	{Key: "X-Content-Type-Options", Value: "nosniff"},
	{Key: "X-DNS-Prefetch-Control", Value: "off"},
	{Key: "X-Download-Options", Value: "noopen"},
	{Key: "X-Frame-Options", Value: "DENY"},
	{Key: "X-Permitted-Cross-Domain-Policies", Value: "none"},
	{Key: "X-XSS-Protection", Value: "0"},
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
		for _, header := range defaultSecurityHeaders {
			headers.Set(header.Key, header.Value)
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
