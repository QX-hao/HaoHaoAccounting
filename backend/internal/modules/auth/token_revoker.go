package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/cache"
)

// tokenRevocationStore 抽象 Redis 能力，方便测试和未启用 Redis 时保持登出接口可用。
type tokenRevocationStore interface {
	Enabled() bool
	Exists(ctx context.Context, key string) (bool, error)
	SetString(ctx context.Context, key, value string, ttl time.Duration) error
}

// TokenRevoker 通过缓存记录已登出的 JWT 摘要，在 token 自然过期前拦截再次使用。
type TokenRevoker struct {
	cache tokenRevocationStore
}

func NewTokenRevoker(redisCache *cache.RedisCache) *TokenRevoker {
	return &TokenRevoker{cache: redisCache}
}

// IsTokenRevoked Redis 未启用时返回未撤销，保持单机/本地开发环境不依赖外部缓存。
func (r *TokenRevoker) IsTokenRevoked(ctx context.Context, token string) (bool, error) {
	if r == nil || r.cache == nil || !r.cache.Enabled() {
		return false, nil
	}
	return r.cache.Exists(ctx, cache.RevokedTokenKey(tokenHash(token)))
}

// RevokeToken 只保存 token 摘要和剩余 TTL，避免把完整 JWT 明文写进缓存。
func (r *TokenRevoker) RevokeToken(ctx context.Context, token string, ttl time.Duration) error {
	if r == nil || r.cache == nil || !r.cache.Enabled() {
		return nil
	}
	if ttl <= 0 {
		return nil
	}
	return r.cache.SetString(ctx, cache.RevokedTokenKey(tokenHash(token)), "1", ttl)
}

// tokenHash 用 SHA-256 生成稳定缓存键，隐藏原始 token 内容。
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
