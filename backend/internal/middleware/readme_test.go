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
		"Strict-Transport-Security",
		"http.MaxBytesReader",
		"application/*+json",
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
