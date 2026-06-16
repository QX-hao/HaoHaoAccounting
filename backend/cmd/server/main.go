package main

import (
	"context"
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
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/middleware"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
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
	applyGlobalMiddleware(r, cfg)

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

func applyGlobalMiddleware(router *gin.Engine, cfg config.Config) {
	router.Use(middleware.RequestID())
	router.Use(middleware.RequestTimeout(cfg.HTTP.RequestTimeout))
	router.Use(gin.LoggerWithConfig(newLoggerConfig()), middleware.Recovery())
	router.Use(middleware.SecurityHeaders(securityHeadersConfig(cfg)))
	router.Use(cors.New(newCORSConfig(cfg)))
	router.Use(middleware.BodyLimit(cfg.HTTP.MaxBodyBytes))
	router.Use(middleware.ContentType(middleware.APIMediaTypeRules()))
	router.Use(middleware.Accept(middleware.APIAcceptRules()))
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
		AllowOrigins: cfg.HTTP.CORSAllowOrigins,
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
			"Content-Disposition",
			"Link",
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
	for _, origin := range cfg.HTTP.CORSAllowOrigins {
		if err := validateExplicitCORSOrigin(origin); err != nil {
			return err
		}
	}

	corsConfig := newCORSConfig(cfg)
	if err := corsConfig.Validate(); err != nil {
		return fmt.Errorf("CORS config: %w", err)
	}
	return nil
}

func validateExplicitCORSOrigin(origin string) error {
	origin = strings.TrimSpace(origin)
	if strings.Contains(origin, "*") {
		return fmt.Errorf("CORS_ALLOW_ORIGINS must use explicit origins; %q contains a wildcard", origin)
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("CORS_ALLOW_ORIGINS contains invalid origin %q", origin)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("CORS_ALLOW_ORIGINS origin %q must use http or https", origin)
	}
	if parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("CORS_ALLOW_ORIGINS origin %q must not include user info, path, query, or fragment", origin)
	}
	return nil
}

func securityHeadersConfig(cfg config.Config) middleware.SecurityHeadersConfig {
	return middleware.SecurityHeadersConfig{
		HSTSMaxAgeSeconds:     cfg.HTTP.HSTSMaxAgeSeconds,
		HSTSIncludeSubDomains: cfg.HTTP.HSTSIncludeSubDomains,
		HSTSPreload:           cfg.HTTP.HSTSPreload,
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
	requestID := "-"
	if value, ok := param.Keys[middleware.RequestIDContextKey]; ok {
		if id, ok := value.(string); ok && id != "" {
			requestID = id
		}
	}

	return fmt.Sprintf(
		"time=%q status=%d latency=%q client_ip=%q method=%q path=%q request_id=%q bytes=%d error=%q\n",
		param.TimeStamp.Format(time.RFC3339),
		param.StatusCode,
		param.Latency.String(),
		param.ClientIP,
		param.Method,
		param.Path,
		requestID,
		param.BodySize,
		param.ErrorMessage,
	)
}
