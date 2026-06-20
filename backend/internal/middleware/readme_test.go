package middleware

import (
	"os"
	"strings"
	"testing"
)

func TestMiddlewareReadmeDocumentsGlobalContracts(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"`RequestID`",
		"`RequestTimeout`",
		"`Recovery`",
		"`SecurityHeaders`",
		"`BodyLimit`",
		"`ContentType`",
		"`Accept`",
		"`NoStore`",
		"`RequireAuth`",
		"128 visible ASCII characters",
		"invalid values are replaced",
		"Content-Security-Policy",
		"Cross-Origin-Opener-Policy",
		"Cross-Origin-Resource-Policy",
		"Origin-Agent-Cluster",
		"Referrer-Policy",
		"Permissions-Policy",
		"X-Content-Type-Options",
		"X-DNS-Prefetch-Control",
		"X-Download-Options",
		"X-Frame-Options",
		"X-Permitted-Cross-Domain-Policies",
		"X-XSS-Protection",
		"Strict-Transport-Security",
		"Broken pipe",
		"connection reset",
		"already-written response",
		"http.MaxBytesReader",
		"application/*+json",
		"structured syntax suffixes",
		"`q=0` media ranges as explicit exclusions",
		"Vary: Accept",
		"Cache-Control: no-store",
		"bearer JWT signature",
		"expiration",
		"issued-at time",
		"issuer",
		"audience",
		"fail closed",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("README.md is missing middleware guidance %q", want)
		}
	}
}
