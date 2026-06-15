package ai

import (
	"context"
	"testing"
)

type aiContextKey struct{}

func TestRequestContextPreservesCallerContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), aiContextKey{}, "request-context")

	if got := requestContext(ctx).Value(aiContextKey{}); got != "request-context" {
		t.Fatalf("context value = %#v", got)
	}
}

func TestRequestContextFallsBackForNil(t *testing.T) {
	if requestContext(nil) == nil {
		t.Fatal("expected fallback context")
	}
}

func TestCacheWriteContextDetachesCancellationAndPreservesValues(t *testing.T) {
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), aiContextKey{}, "request-context"))
	cancel()

	ctx := cacheWriteContext(parent)
	if err := ctx.Err(); err != nil {
		t.Fatalf("cache write context error = %v", err)
	}
	if got := ctx.Value(aiContextKey{}); got != "request-context" {
		t.Fatalf("context value = %#v", got)
	}
}

func TestCacheWriteContextFallsBackForNil(t *testing.T) {
	if cacheWriteContext(nil) == nil {
		t.Fatal("expected fallback context")
	}
}

func TestParseAcceptsNilContext(t *testing.T) {
	resp := NewService(nil).Parse(nil, 1, "午饭 20 元")

	if !resp.RequiresConfirmation {
		t.Fatal("expected confirmation")
	}
	if resp.Cached {
		t.Fatal("expected uncached response without redis")
	}
}
