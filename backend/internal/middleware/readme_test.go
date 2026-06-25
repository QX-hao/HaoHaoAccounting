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
		"`499 client_closed_request` response",
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
		"Cross-Origin-Embedder-Policy",
		"opt-in through HTTP config",
		"cross-origin isolation",
		"Broken pipe",
		"connection reset",
		"already-written response",
		"http.MaxBytesReader",
		"application/*+json",
		"bare concrete types without parameters or wildcards",
		"normalized, deduplicated",
		"invalid values are ignored",
		"structured syntax suffixes",
		"`q=0` media ranges as explicit exclusions",
		"Vary: Accept",
		"duplicate or invalid rule entries",
		"low-cardinality Prometheus counters and duration histograms",
		"Gin route pattern",
		"unmatched",
		"Cache-Control: no-store",
		"API prefix",
		"early global rejections",
		"`SetNoCache`",
		"health probes",
		"revalidated before reuse",
		"bearer JWT signature",
		"RFC 6750 token68 character set",
		"at most 4096 bytes",
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
	for _, header := range defaultSecurityHeaders {
		if !strings.Contains(source, header.Key) {
			t.Fatalf("README.md is missing default security header %q", header.Key)
		}
	}
}
