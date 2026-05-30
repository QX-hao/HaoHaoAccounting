package transactions

import (
	"context"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/cache"
)

type CacheInvalidator interface {
	InvalidateUser(userID uint)
}

type redisInvalidator struct {
	cache *cache.RedisCache
}

func NewCacheInvalidator(redisCache *cache.RedisCache) CacheInvalidator {
	return &redisInvalidator{cache: redisCache}
}

func (r *redisInvalidator) InvalidateUser(userID uint) {
	if r.cache == nil || !r.cache.Enabled() {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = r.cache.DeleteByPrefix(ctx, cache.UserReportPrefix(userID))
}

type noopInvalidator struct{}

func (noopInvalidator) InvalidateUser(userID uint) {}
