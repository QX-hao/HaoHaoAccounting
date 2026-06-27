package app

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/cache"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/config"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
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

type pinger interface {
	Ping(context.Context) error
}

// RegisterRoutes 是 HTTP 路由树的统一入口；模块负责 handler 和业务逻辑，这里只负责组合。
func RegisterRoutes(engine *gin.Engine, s *store.Store, redisCache *cache.RedisCache) error {
	return RegisterRoutesWithConfig(engine, s, redisCache, config.Load())
}

// RegisterRoutesWithConfig 注册健康检查、API 分组、公开登录路由和受 Bearer 认证保护的私有路由。
func RegisterRoutesWithConfig(engine *gin.Engine, s *store.Store, redisCache *cache.RedisCache, cfg config.Config) error {
	configureRouter(engine)
	registerFallbackRoutes(engine)
	registerHealthRoutes(engine, s, redisCache)

	api := engine.Group("/api/v1")
	api.Use(middleware.NoStore())

	if err := cfg.JWT.Validate(); err != nil {
		return err
	}
	tokenService, err := middleware.NewTokenServiceWithTTL(cfg.JWT.Secret, cfg.JWT.TTL, cfg.JWT.ClockSkew, cfg.JWT.Issuer, cfg.JWT.Audience)
	if err != nil {
		return err
	}

	authHandler := auth.NewHandlerWithConfig(s, cfg.Admin, cfg.LoginRateLimit, tokenService)
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
	authGroup.Use(middleware.RequireAuthWithRevocation(tokenRevoker, tokenService))

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

func configureRouter(engine *gin.Engine) {
	// API 路径以 OpenAPI 为准；关闭 Gin 默认路径修复，避免绕过统一的结构化错误响应。
	engine.RedirectTrailingSlash = false
	engine.RedirectFixedPath = false
	engine.RemoveExtraSlash = false
	// 上传文件的业务上限是 5 MiB；multipart 解析内存预算也保持同一量级，避免依赖 Gin 默认 32 MiB。
	engine.MaxMultipartMemory = dataio.MaxImportFileBytes
}

func registerFallbackRoutes(engine *gin.Engine) {
	engine.HandleMethodNotAllowed = true
	engine.NoRoute(func(c *gin.Context) {
		noStoreAPIError(c)
		httputil.NotFound(c, "route not found")
	})
	engine.NoMethod(func(c *gin.Context) {
		noStoreAPIError(c)
		normalizeAllowHeader(c.Writer.Header())
		httputil.MethodNotAllowed(c, "method not allowed")
	})
}

func noStoreAPIError(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/v1/") || c.Request.URL.Path == "/api/v1" {
		middleware.SetNoStore(c.Writer.Header())
	}
}

// normalizeAllowHeader 固定 405 Allow 响应头顺序，避免 Gin 内部路由树遍历顺序泄漏成 API 契约。
func normalizeAllowHeader(headers http.Header) {
	methods := parseAllowHeader(headers.Get("Allow"))
	if len(methods) == 0 {
		return
	}
	headers.Set("Allow", strings.Join(methods, ", "))
}

func parseAllowHeader(value string) []string {
	seen := make(map[string]struct{})
	methods := make([]string, 0)
	for _, part := range strings.Split(value, ",") {
		method := strings.ToUpper(strings.TrimSpace(part))
		if method == "" {
			continue
		}
		if _, ok := seen[method]; ok {
			continue
		}
		seen[method] = struct{}{}
		methods = append(methods, method)
	}
	slices.SortFunc(methods, compareHTTPMethods)
	return methods
}

func compareHTTPMethods(left, right string) int {
	leftRank := httpMethodRank(left)
	rightRank := httpMethodRank(right)
	if leftRank != rightRank {
		return leftRank - rightRank
	}
	return strings.Compare(left, right)
}

func httpMethodRank(method string) int {
	for i, candidate := range []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions,
	} {
		if method == candidate {
			return i
		}
	}
	return 100
}

func registerHealthRoutes(engine *gin.Engine, s *store.Store, redisCache *cache.RedisCache) {
	readyzHandler := readyz(s, redisCache)
	// 探针同时支持 GET 和 HEAD，方便负载均衡器只检查状态码和响应头，不拉取 JSON body。
	engine.GET("/livez", livez)
	engine.HEAD("/livez", livez)
	engine.GET("/readyz", readyzHandler)
	engine.HEAD("/readyz", readyzHandler)
	engine.GET("/health", readyzHandler)
	engine.HEAD("/health", readyzHandler)
}

func livez(c *gin.Context) {
	middleware.SetNoCache(c.Writer.Header())
	if c.Request.Method == http.MethodHead {
		c.Status(http.StatusOK)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func readyz(s *store.Store, redisCache *cache.RedisCache) gin.HandlerFunc {
	var redis pinger
	if redisCache != nil && redisCache.Enabled() {
		redis = redisCache
	}
	return readyzWithDependencies(s, redis)
}

func readyzWithDependencies(database pinger, redis pinger) gin.HandlerFunc {
	return func(c *gin.Context) {
		middleware.SetNoCache(c.Writer.Header())

		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		checks := gin.H{}
		status := http.StatusOK

		if database == nil {
			status = http.StatusServiceUnavailable
			checks["database"] = dependencyStatus("error", errors.New("database is not configured"))
		} else if err := database.Ping(ctx); err != nil {
			status = http.StatusServiceUnavailable
			checks["database"] = dependencyStatus("error", err)
		} else {
			checks["database"] = dependencyStatus("ok", nil)
		}

		if redis == nil {
			checks["redis"] = gin.H{"status": "disabled"}
		} else if err := redis.Ping(ctx); err != nil {
			status = http.StatusServiceUnavailable
			checks["redis"] = dependencyStatus("error", err)
		} else {
			checks["redis"] = dependencyStatus("ok", nil)
		}

		overall := "ok"
		if status != http.StatusOK {
			overall = "unavailable"
		}
		if c.Request.Method == http.MethodHead {
			c.Status(status)
			return
		}
		c.JSON(status, gin.H{
			"status": overall,
			"checks": checks,
		})
	}
}

func dependencyStatus(status string, err error) gin.H {
	result := gin.H{"status": status}
	if err != nil {
		// 健康探针只暴露依赖状态，避免把 DSN、主机名或内部网络错误回传给外部探针。
		result["error"] = "unavailable"
	}
	return result
}
