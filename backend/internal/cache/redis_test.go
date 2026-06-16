package cache

import (
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
