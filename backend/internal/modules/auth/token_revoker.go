package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/cache"
)

type TokenRevoker struct {
	cache *cache.RedisCache
}

func NewTokenRevoker(redisCache *cache.RedisCache) *TokenRevoker {
	return &TokenRevoker{cache: redisCache}
}

func (r *TokenRevoker) IsTokenRevoked(ctx context.Context, token string) (bool, error) {
	if r == nil || r.cache == nil || !r.cache.Enabled() {
		return false, nil
	}
	return r.cache.Exists(ctx, cache.RevokedTokenKey(tokenHash(token)))
}

func (r *TokenRevoker) RevokeToken(ctx context.Context, token string) error {
	if r == nil || r.cache == nil || !r.cache.Enabled() {
		return nil
	}
	return r.cache.SetString(ctx, cache.RevokedTokenKey(tokenHash(token)), "1", 8*24*time.Hour)
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
