package ai

import (
	"context"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/cache"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/services"
)

type ParseResponse struct {
	RequiresConfirmation bool                   `json:"requiresConfirmation"`
	Cached               bool                   `json:"cached"`
	Result               services.AIParseResult `json:"result"`
}

type Service struct {
	cache *cache.RedisCache
}

func NewService(redisCache *cache.RedisCache) *Service {
	return &Service{cache: redisCache}
}

func (s *Service) Parse(ctx context.Context, userID uint, text string) ParseResponse {
	cacheKey := cache.UserAIParseKey(userID, text)

	if s.cache != nil && s.cache.Enabled() {
		cacheCtx, cancel := context.WithTimeout(requestContext(ctx), time.Second)
		defer cancel()

		var cached services.AIParseResult
		ok, err := s.cache.GetJSON(cacheCtx, cacheKey, &cached)
		if err == nil && ok {
			return ParseResponse{RequiresConfirmation: true, Cached: true, Result: cached}
		}
	}

	parsed := services.ParseNaturalLedgerText(text)
	if s.cache != nil && s.cache.Enabled() {
		cacheCtx, cancel := context.WithTimeout(cacheWriteContext(ctx), time.Second)
		defer cancel()
		_ = s.cache.SetJSON(cacheCtx, cacheKey, parsed, 10*time.Minute)
	}

	return ParseResponse{RequiresConfirmation: true, Cached: false, Result: parsed}
}

func requestContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func cacheWriteContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}
