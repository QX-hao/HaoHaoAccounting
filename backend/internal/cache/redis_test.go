package cache

import (
	"context"
	"sync"
	"testing"

	"github.com/redis/go-redis/v9"
)

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
