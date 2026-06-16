package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Addr     string
	Password string
	DB       int
}

type RedisCache struct {
	mu     sync.RWMutex
	client *redis.Client
}

func New(cfg Config) (*RedisCache, error) {
	if cfg.Addr == "" {
		return nil, errors.New("redis addr is empty")
	}

	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return &RedisCache{client: client}, nil
}

func (c *RedisCache) Enabled() bool {
	return c.clientOrNil() != nil
}

func (c *RedisCache) Ping(ctx context.Context) error {
	client := c.clientOrNil()
	if client == nil {
		return nil
	}
	return client.Ping(ctx).Err()
}

func (c *RedisCache) Close() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	client := c.client
	c.client = nil
	c.mu.Unlock()
	if client == nil {
		return nil
	}
	return client.Close()
}

func (c *RedisCache) GetJSON(ctx context.Context, key string, out any) (bool, error) {
	client := c.clientOrNil()
	if client == nil {
		return false, nil
	}

	val, err := client.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(val), out); err != nil {
		return false, err
	}
	return true, nil
}

func (c *RedisCache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	client := c.clientOrNil()
	if client == nil {
		return nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return client.Set(ctx, key, payload, ttl).Err()
}

func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	client := c.clientOrNil()
	if client == nil {
		return false, nil
	}
	count, err := client.Exists(ctx, key).Result()
	return count > 0, err
}

func (c *RedisCache) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	client := c.clientOrNil()
	if client == nil {
		return nil
	}
	return client.Set(ctx, key, value, ttl).Err()
}

func (c *RedisCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	client := c.clientOrNil()
	if client == nil {
		return nil
	}

	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, prefix+"*", 200).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (c *RedisCache) clientOrNil() *redis.Client {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.client
}

func UserReportPrefix(userID uint) string {
	return fmt.Sprintf("report:summary:u:%d:", userID)
}

func UserReportKey(userID uint, start, end time.Time) string {
	return fmt.Sprintf("report:summary:u:%d:%s:%s", userID, start.Format(time.RFC3339), end.Format(time.RFC3339))
}

func UserAIParseKey(userID uint, text string) string {
	return fmt.Sprintf("ai:parse:u:%d:%x", userID, text)
}

func RevokedTokenKey(tokenHash string) string {
	return fmt.Sprintf("auth:revoked:%s", tokenHash)
}
