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
		// 事务写入已经成功后，即使客户端断开，也应尽量清理该用户的报表缓存。
		ctx = context.WithoutCancel(ctx)
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	// 缓存失效失败不影响主写入路径；下一次 TTL 到期后仍会自动恢复一致。
	_ = r.cache.DeleteByPrefix(ctx, cache.UserReportPrefix(userID))
}

type noopInvalidator struct{}

func (noopInvalidator) InvalidateUser(context.Context, uint) {}
