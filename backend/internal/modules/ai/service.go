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

func (s *Service) Parse(userID uint, text string) ParseResponse {
	cacheKey := cache.UserAIParseKey(userID, text)

	if s.cache != nil && s.cache.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		var cached services.AIParseResult
		ok, err := s.cache.GetJSON(ctx, cacheKey, &cached)
		if err == nil && ok {
			return ParseResponse{RequiresConfirmation: true, Cached: true, Result: cached}
		}
	}

	parsed := services.ParseNaturalLedgerText(text)
	if s.cache != nil && s.cache.Enabled() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.cache.SetJSON(ctx, cacheKey, parsed, 10*time.Minute)
	}

	return ParseResponse{RequiresConfirmation: true, Cached: false, Result: parsed}
}
