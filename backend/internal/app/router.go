package app

import (
	"net/http"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/cache"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/accounts"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/ai"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/auth"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/budgets"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/categories"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/dataio"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/reports"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/modules/transactions"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes is the only place that knows the public HTTP route tree.
// Modules own their handlers and business behavior; this layer only composes them.
func RegisterRoutes(engine *gin.Engine, s *store.Store, redisCache *cache.RedisCache) error {
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":       "ok",
			"redisEnabled": redisCache != nil && redisCache.Enabled(),
		})
	})

	api := engine.Group("/api/v1")

	authHandler := auth.NewHandler(s)
	if err := authHandler.EnsureBootstrapAdmin(); err != nil {
		return err
	}
	authHandler.RegisterPublic(api)

	tokenRevoker := auth.NewTokenRevoker(redisCache)
	authGroup := api.Group("")
	authGroup.Use(func(c *gin.Context) {
		c.Set("token_revoker", tokenRevoker)
		c.Next()
	})
	authGroup.Use(middleware.RequireAuthWithRevocation(tokenRevoker))

	cacheInvalidator := transactions.NewCacheInvalidator(redisCache)
	transactionService := transactions.NewService(s, cacheInvalidator)

	authHandler.RegisterPrivate(authGroup)
	accounts.NewHandler(accounts.NewService(s, cacheInvalidator)).Register(authGroup)
	budgets.NewHandler(budgets.NewService(s, cacheInvalidator)).Register(authGroup)
	categories.NewHandler(categories.NewService(s, cacheInvalidator)).Register(authGroup)
	transactionHandler := transactions.NewHandler(transactionService)
	transactionHandler.Register(authGroup)
	ai.NewHandler(ai.NewService(redisCache)).Register(authGroup)
	reports.NewHandler(reports.NewService(s, redisCache)).Register(authGroup)
	dataio.NewHandler(dataio.NewService(s, transactionService, cacheInvalidator)).Register(authGroup)
	return nil
}
