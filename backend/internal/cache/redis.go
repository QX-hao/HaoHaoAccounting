package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Addr     string
	Password string
	DB       int
}

type RedisCache struct {
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
		return nil, err
	}

	return &RedisCache{client: client}, nil
}

func (c *RedisCache) Enabled() bool {
	return c != nil && c.client != nil
}

func (c *RedisCache) GetJSON(ctx context.Context, key string, out any) (bool, error) {
	if !c.Enabled() {
		return false, nil
	}

	val, err := c.client.Get(ctx, key).Result()
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
	if !c.Enabled() {
		return nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, payload, ttl).Err()
}

func (c *RedisCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	if !c.Enabled() {
		return nil
	}

	var cursor uint64
	for {
		keys, next, err := c.client.Scan(ctx, cursor, prefix+"*", 200).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
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

func UserReportPrefix(userID uint) string {
	return fmt.Sprintf("report:summary:u:%d:", userID)
}

func UserReportKey(userID uint, start, end time.Time) string {
	return fmt.Sprintf("report:summary:u:%d:%s:%s", userID, start.Format(time.RFC3339), end.Format(time.RFC3339))
}

func UserAIParseKey(userID uint, text string) string {
	return fmt.Sprintf("ai:parse:u:%d:%x", userID, text)
}
