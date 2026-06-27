package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestNewRequiresRedisAddress(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatal("expected empty redis addr to fail")
	}
}

func TestRedisCacheCloseDisablesClientAndIsIdempotent(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	c := &RedisCache{client: client}

	if !c.Enabled() {
		t.Fatal("expected redis cache to be enabled before Close")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if c.Enabled() {
		t.Fatal("Close did not disable redis cache")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	var nilCache *RedisCache
	if err := nilCache.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
}

func TestRedisCacheOperationsAreNoopsAfterClose(t *testing.T) {
	c := &RedisCache{client: redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ctx := context.Background()
	if err := c.Ping(ctx); err != nil {
		t.Fatalf("Ping after close: %v", err)
	}
	var out map[string]any
	if ok, err := c.GetJSON(ctx, "key", &out); ok || err != nil {
		t.Fatalf("GetJSON after close = %v, %v", ok, err)
	}
	if err := c.SetJSON(ctx, "key", map[string]string{"ok": "true"}, 0); err != nil {
		t.Fatalf("SetJSON after close: %v", err)
	}
	if ok, err := c.Exists(ctx, "key"); ok || err != nil {
		t.Fatalf("Exists after close = %v, %v", ok, err)
	}
	if err := c.SetString(ctx, "key", "value", 0); err != nil {
		t.Fatalf("SetString after close: %v", err)
	}
	if err := c.DeleteByPrefix(ctx, "key"); err != nil {
		t.Fatalf("DeleteByPrefix after close: %v", err)
	}
}

func TestRedisCacheDeleteByPrefixRejectsEmptyPrefix(t *testing.T) {
	c := &RedisCache{client: redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})}
	defer func() { _ = c.Close() }()

	if err := c.DeleteByPrefix(context.Background(), ""); !errors.Is(err, errEmptyDeletePrefix) {
		t.Fatalf("DeleteByPrefix empty prefix = %v, want %v", err, errEmptyDeletePrefix)
	}
}

func TestRedisCacheSetJSONReturnsMarshalError(t *testing.T) {
	c := &RedisCache{client: redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})}
	defer func() { _ = c.Close() }()

	value := map[string]any{"invalid": func() {}}
	if err := c.SetJSON(context.Background(), "key", value, time.Minute); err == nil {
		t.Fatal("expected marshal error")
	}
}

func TestRedisCacheCloseIsSafeWithConcurrentEnabledChecks(t *testing.T) {
	c := &RedisCache{client: redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = c.Enabled()
			}
		}()
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	wg.Wait()
	if c.Enabled() {
		t.Fatal("expected cache to be disabled after Close")
	}
}

func TestCacheKeysAreStableAndScoped(t *testing.T) {
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	end := time.Date(2026, 1, 3, 3, 4, 5, 0, time.UTC)

	tests := map[string]string{
		"report prefix": UserReportPrefix(42),
		"report key":    UserReportKey(42, start, end),
		"ai parse key":  UserAIParseKey(42, "记一笔早餐12.5"),
		"revoked token": RevokedTokenKey("abc123"),
	}
	want := map[string]string{
		"report prefix": "report:summary:u:42:",
		"report key":    "report:summary:u:42:2026-01-02T03:04:05Z:2026-01-03T03:04:05Z",
		"ai parse key":  "ai:parse:u:42:e8aeb0e4b880e7ac94e697a9e9a49031322e35",
		"revoked token": "auth:revoked:abc123",
	}
	for name, got := range tests {
		if got != want[name] {
			t.Fatalf("%s key = %q, want %q", name, got, want[name])
		}
	}
}
