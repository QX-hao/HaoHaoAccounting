package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/cache"
)

type tokenRevocationStore interface {
	Enabled() bool
	Exists(ctx context.Context, key string) (bool, error)
	SetString(ctx context.Context, key, value string, ttl time.Duration) error
}

type TokenRevoker struct {
	cache tokenRevocationStore
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

func (r *TokenRevoker) RevokeToken(ctx context.Context, token string, ttl time.Duration) error {
	if r == nil || r.cache == nil || !r.cache.Enabled() {
		return nil
	}
	if ttl <= 0 {
		return nil
	}
	return r.cache.SetString(ctx, cache.RevokedTokenKey(tokenHash(token)), "1", ttl)
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
