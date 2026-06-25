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

var errEmptyDeletePrefix = errors.New("redis delete prefix is empty")

// Config 保存 Redis 连接配置，只暴露当前项目实际需要的地址、密码和 DB 编号。
type Config struct {
	Addr     string
	Password string
	DB       int
}

// RedisCache 封装 go-redis 客户端，让业务模块只依赖项目内的缓存语义。
type RedisCache struct {
	mu     sync.RWMutex
	client *redis.Client
}

// New 创建 Redis 缓存客户端，并在启动阶段用 Ping 提前验证连接可用性。
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

// Enabled 返回缓存当前是否可用；Close 后会变为 false，方便业务路径降级为无缓存。
func (c *RedisCache) Enabled() bool {
	return c.clientOrNil() != nil
}

// Ping 检查 Redis 连接健康；缓存未启用或已关闭时返回 nil，避免健康检查误报。
func (c *RedisCache) Ping(ctx context.Context) error {
	client := c.clientOrNil()
	if client == nil {
		return nil
	}
	return client.Ping(ctx).Err()
}

// Close 关闭 Redis 客户端并清空引用；重复调用或 nil 接收者调用都保持安全。
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

// GetJSON 读取 JSON 缓存并反序列化到 out；未命中或缓存关闭时返回 ok=false。
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

// SetJSON 把 value 序列化为 JSON 后写入 Redis，ttl 由调用方按业务场景决定。
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

// Exists 判断 key 是否存在；缓存关闭时返回 false，保持业务逻辑可降级。
func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	client := c.clientOrNil()
	if client == nil {
		return false, nil
	}
	count, err := client.Exists(ctx, key).Result()
	return count > 0, err
}

// SetString 写入字符串值，适合 token 撤销这类只需要存在性判断的缓存。
func (c *RedisCache) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	client := c.clientOrNil()
	if client == nil {
		return nil
	}
	return client.Set(ctx, key, value, ttl).Err()
}

// DeleteByPrefix 使用 SCAN 分批删除指定前缀的 key，避免生产环境使用阻塞式 KEYS。
func (c *RedisCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	client := c.clientOrNil()
	if client == nil {
		return nil
	}
	if prefix == "" {
		return errEmptyDeletePrefix
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

// UserReportPrefix 返回某个用户全部报表缓存共享的前缀，用于交易变更后的批量失效。
func UserReportPrefix(userID uint) string {
	return fmt.Sprintf("report:summary:u:%d:", userID)
}

// UserReportKey 返回报表汇总缓存的基础 key；过滤条件后缀由报表模块追加。
func UserReportKey(userID uint, start, end time.Time) string {
	return fmt.Sprintf("report:summary:u:%d:%s:%s", userID, start.Format(time.RFC3339), end.Format(time.RFC3339))
}

// UserAIParseKey 返回自然语言解析结果缓存 key，按用户和原始文本隔离。
func UserAIParseKey(userID uint, text string) string {
	return fmt.Sprintf("ai:parse:u:%d:%x", userID, text)
}

// RevokedTokenKey 返回 JWT 撤销列表 key；调用方应传入 token 摘要而不是明文 token。
func RevokedTokenKey(tokenHash string) string {
	return fmt.Sprintf("auth:revoked:%s", tokenHash)
}
