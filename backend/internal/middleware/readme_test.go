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
		"structured `504` timeout response",
		"`Recovery`",
		"`SecurityHeaders`",
		"`BodyLimit`",
		"`ContentType`",
		"`Accept`",
		"`HTTPMetrics`",
		"`NoStoreAPI`",
		"`NoStore`",
		"`RequireAuth`",
		"128 visible ASCII characters",
		"invalid values are replaced",
		"Strict-Transport-Security",
		"Broken pipe",
		"connection reset",
		"already-written response",
		"http.MaxBytesReader",
		"application/*+json",
		"structured syntax suffixes",
		"`q=0` media ranges as explicit exclusions",
		"Vary: Accept",
		"low-cardinality Prometheus counters and duration histograms",
		"Gin route pattern",
		"unmatched",
		"Cache-Control: no-store",
		"API prefix",
		"early global rejections",
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
	for header := range defaultSecurityHeaders() {
		if !strings.Contains(source, header) {
			t.Fatalf("README.md is missing default security header %q", header)
		}
	}
}
