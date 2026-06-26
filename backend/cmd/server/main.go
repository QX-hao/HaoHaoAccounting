package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/app"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/cache"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/config"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/shared/stringutil"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatalf("%v", err)
	}
}

func run(parent context.Context) error {
	if err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("skip .env: %v", err)
	}
	cfg, err := config.LoadStrict()
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if err := validateStartupConfig(cfg); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	applyGinMode(cfg)

	s, err := store.New(storeConfig(cfg.Database))
	if err != nil {
		return fmt.Errorf("failed to init store: %w", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	var redisCache *cache.RedisCache
	if c, err := cache.New(cache.Config{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}); err != nil {
		log.Printf("redis disabled: %v", err)
	} else {
		redisCache = c
		defer func() {
			if err := redisCache.Close(); err != nil {
				log.Printf("close redis: %v", err)
			}
		}()
	}

	r := gin.New()
	if err := r.SetTrustedProxies(cfg.HTTP.TrustedProxies); err != nil {
		return fmt.Errorf("failed to set trusted proxies: %w", err)
	}
	applyGlobalMiddleware(r, cfg, installMetrics(r, cfg))

	if err := app.RegisterRoutesWithConfig(r, s, redisCache, cfg); err != nil {
		return fmt.Errorf("failed to register routes: %w", err)
	}

	log.Printf(
		"server running at :%s with DB_DRIVER=%s redis=%t",
		cfg.Port,
		cfg.Database.Driver,
		redisCache != nil && redisCache.Enabled(),
	)
	server := newHTTPServer(cfg, r)
	if err := runHTTPServer(parent, server, cfg.HTTP.ShutdownTimeout); err != nil {
		return fmt.Errorf("server exit: %w", err)
	}
	return nil
}

func newHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		MaxHeaderBytes:    cfg.HTTP.MaxHeaderBytes,
	}
}

func applyGlobalMiddleware(router *gin.Engine, cfg config.Config, metrics *middleware.HTTPMetrics) {
	// 中间件顺序会影响错误响应能否带上 request id、安全头、CORS 和 no-store。
	// 先放 RequestID 和指标采集，后面的早期拒绝也能被追踪和统计。
	router.Use(middleware.RequestID())
	if metrics != nil {
		router.Use(metrics.Middleware())
	}
	router.Use(middleware.RequestTimeout(cfg.HTTP.RequestTimeout))
	router.Use(gin.LoggerWithConfig(newLoggerConfig()), middleware.Recovery())
	router.Use(middleware.SecurityHeaders(securityHeadersConfig(cfg)))
	router.Use(cors.New(newCORSConfig(cfg)))
	router.Use(middleware.NoStoreAPI("/api/v1"))
	router.Use(middleware.BodyLimit(cfg.HTTP.MaxBodyBytes))
	router.Use(middleware.ContentType(middleware.APIMediaTypeRules()))
	router.Use(middleware.Accept(middleware.APIAcceptRules()))
}

func installMetrics(router *gin.Engine, cfg config.Config) *middleware.HTTPMetrics {
	if !cfg.HTTP.MetricsEnabled {
		return nil
	}
	metricsRegistry := newMetricsRegistry()
	// /metrics 在全局中间件挂载前注册，避免应用 API 的 Accept/Content-Type 规则影响 Prometheus 抓取。
	registerMetricsRoute(router, metricsRegistry, cfg)
	return middleware.NewHTTPMetrics(metricsRegistry)
}

func registerMetricsRoute(router *gin.Engine, registry *prometheus.Registry, cfg config.Config) {
	// HandlerOpts.Registry 会把 promhttp_metric_handler_errors_total 注册进同一个 registry，方便告警发现采集/编码失败。
	handler := gin.WrapH(promhttp.InstrumentMetricHandler(registry, promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry})))
	handlers := []gin.HandlerFunc{
		middleware.RequestID(),
		middleware.Recovery(),
		middleware.SecurityHeaders(securityHeadersConfig(cfg)),
	}
	if cfg.HTTP.MetricsToken != "" {
		handlers = append(handlers, requireMetricsToken(cfg.HTTP.MetricsToken))
	}
	handlers = append(handlers, handler)
	router.GET("/metrics", handlers...)
}

func newMetricsRegistry() *prometheus.Registry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return registry
}

func requireMetricsToken(want string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := middleware.BearerToken(c.GetHeader("Authorization"))
		if !ok || !constantTimeTokenEqual(token, want) {
			httputil.Unauthorized(c, "invalid metrics token")
			c.Abort()
			return
		}
		c.Next()
	}
}

func constantTimeTokenEqual(got, want string) bool {
	// 先哈希成固定长度摘要再比较，避免长度不同导致 ConstantTimeCompare 提前返回。
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func validateStartupConfig(cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := validateCORSConfig(cfg); err != nil {
		return err
	}
	return nil
}

func applyGinMode(cfg config.Config) {
	gin.SetMode(cfg.HTTP.GinMode)
}

func storeConfig(cfg config.DatabaseConfig) store.Config {
	return store.Config{
		Driver:          cfg.Driver,
		DSN:             cfg.DSN,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.ConnMaxIdleTime,
	}
}

func newCORSConfig(cfg config.Config) cors.Config {
	return cors.Config{
		AllowOrigins: normalizedCORSOrigins(cfg.HTTP.CORSAllowOrigins),
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			"Origin",
			"Content-Type",
			"Accept",
			"Authorization",
			middleware.RequestIDHeader,
		},
		ExposeHeaders: []string{
			"Allow",
			"Clear-Site-Data",
			"Content-Disposition",
			"Link",
			"Location",
			"RateLimit-Limit",
			"RateLimit-Remaining",
			"RateLimit-Reset",
			"WWW-Authenticate",
			"Retry-After",
			"X-Total-Count",
			middleware.RequestIDHeader,
		},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
}

func validateCORSConfig(cfg config.Config) error {
	origins := normalizedCORSOrigins(cfg.HTTP.CORSAllowOrigins)
	for _, origin := range origins {
		if err := validateExplicitCORSOrigin(origin); err != nil {
			return err
		}
	}

	corsConfig := newCORSConfig(config.Config{HTTP: config.HTTPConfig{CORSAllowOrigins: origins}})
	if err := corsConfig.Validate(); err != nil {
		return fmt.Errorf("CORS config: %w", err)
	}
	return nil
}

func normalizedCORSOrigins(origins []string) []string {
	result := make([]string, 0, len(origins))
	seen := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		clean := strings.TrimSpace(origin)
		if clean == "" {
			continue
		}
		normalized, err := canonicalCORSOrigin(clean)
		if err != nil {
			normalized = clean
		}
		if normalized != "" {
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			result = append(result, normalized)
		}
	}
	return result
}

func validateExplicitCORSOrigin(origin string) error {
	_, err := canonicalCORSOrigin(origin)
	return err
}

// canonicalCORSOrigin 转成浏览器 Origin 头常见的序列化形式，避免大小写或默认端口导致精确匹配失败。
func canonicalCORSOrigin(origin string) (string, error) {
	origin = strings.TrimSpace(origin)
	if strings.Contains(origin, "*") {
		return "", fmt.Errorf("CORS_ALLOW_ORIGINS must use explicit origins; %q contains a wildcard", origin)
	}

	parsed, err := url.ParseRequestURI(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("CORS_ALLOW_ORIGINS contains invalid origin %q", origin)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("CORS_ALLOW_ORIGINS origin %q must use http or https", origin)
	}
	if parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("CORS_ALLOW_ORIGINS origin %q must not include user info, path, query, or fragment", origin)
	}

	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		host += ":" + port
	}
	return scheme + "://" + host, nil
}

func securityHeadersConfig(cfg config.Config) middleware.SecurityHeadersConfig {
	return middleware.SecurityHeadersConfig{
		HSTSMaxAgeSeconds:         cfg.HTTP.HSTSMaxAgeSeconds,
		HSTSIncludeSubDomains:     cfg.HTTP.HSTSIncludeSubDomains,
		HSTSPreload:               cfg.HTTP.HSTSPreload,
		CrossOriginEmbedderPolicy: cfg.HTTP.CrossOriginEmbedderPolicy,
	}
}

func newLoggerConfig() gin.LoggerConfig {
	return gin.LoggerConfig{
		Formatter: requestLogFormatter,
		SkipPaths: []string{
			"/livez",
			"/readyz",
			"/health",
		},
	}
}

func runHTTPServer(parent context.Context, server *http.Server, shutdownTimeout time.Duration) error {
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return <-errCh
}

func requestLogFormatter(param gin.LogFormatterParams) string {
	if param.Latency > time.Minute {
		param.Latency = param.Latency.Truncate(time.Second)
	}

	return fmt.Sprintf(
		"time=%q status=%d latency=%q client_ip=%q method=%q path=%q proto=%q user_agent=%q request_id=%q bytes=%d error=%q\n",
		param.TimeStamp.Format(time.RFC3339),
		param.StatusCode,
		param.Latency.String(),
		param.ClientIP,
		param.Method,
		logPath(param.Path),
		logProto(param.Request),
		logUserAgent(param.Request),
		logRequestID(param.Keys),
		param.BodySize,
		logErrorMessage(param.ErrorMessage),
	)
}

const (
	maxLoggedUserAgentLength = 256
	maxLoggedErrorLength     = 512
)

func logProto(request *http.Request) string {
	if request == nil || request.Proto == "" {
		return "-"
	}
	return request.Proto
}

func logUserAgent(request *http.Request) string {
	if request == nil {
		return "-"
	}
	userAgent := strings.TrimSpace(request.UserAgent())
	if userAgent == "" {
		return "-"
	}
	return stringutil.TruncateRunes(userAgent, maxLoggedUserAgentLength)
}

func logPath(path string) string {
	if beforeQuery, _, ok := strings.Cut(path, "?"); ok {
		path = beforeQuery
	}
	if path == "" {
		return "/"
	}
	return path
}

func logRequestID(keys map[string]any) string {
	if value, ok := keys[middleware.RequestIDContextKey]; ok {
		if id, ok := value.(string); ok && middleware.ValidRequestID(id) {
			return id
		}
	}
	return "-"
}

func logErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	return stringutil.TruncateRunes(message, maxLoggedErrorLength)
}
