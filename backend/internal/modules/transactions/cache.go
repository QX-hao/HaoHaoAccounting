package transactions

import (
	"context"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/cache"
)

type CacheInvalidator interface {
	InvalidateUser(ctx context.Context, userID uint)
}

type redisInvalidator struct {
	cache *cache.RedisCache
}

func NewCacheInvalidator(redisCache *cache.RedisCache) CacheInvalidator {
	return &redisInvalidator{cache: redisCache}
}

func (r *redisInvalidator) InvalidateUser(ctx context.Context, userID uint) {
	if r.cache == nil || !r.cache.Enabled() {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	} else {
		ctx = context.WithoutCancel(ctx)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_ = r.cache.DeleteByPrefix(ctx, cache.UserReportPrefix(userID))
}

type noopInvalidator struct{}

func (noopInvalidator) InvalidateUser(context.Context, uint) {}
