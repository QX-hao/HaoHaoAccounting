package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/app"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/cache"
	"github.com/QX-hao/HaoHaoAccounting/backend/internal/store"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	driver := fallbackEnv("DB_DRIVER", "postgres")
	dsn := os.Getenv("DB_DSN")
	if strings.TrimSpace(dsn) == "" {
		if strings.EqualFold(driver, "mysql") {
			dsn = "root:root@tcp(127.0.0.1:53306)/haohaoaccounting?charset=utf8mb4&parseTime=True&loc=Local"
		} else {
			dsn = "host=127.0.0.1 user=postgres password=haohao123 dbname=haohaoaccounting port=55432 sslmode=disable TimeZone=Asia/Shanghai"
		}
	}

	s, err := store.New(store.Config{Driver: driver, DSN: dsn})
	if err != nil {
		log.Fatalf("failed to init store: %v", err)
	}

	redisAddr := fallbackEnv("REDIS_ADDR", "127.0.0.1:56379")
	redisPassword := fallbackEnv("REDIS_PASSWORD", "haohao123")
	redisDB := fallbackIntEnv("REDIS_DB", 0)

	var redisCache *cache.RedisCache
	if c, err := cache.New(cache.Config{
		Addr:     redisAddr,
		Password: redisPassword,
		DB:       redisDB,
	}); err != nil {
		log.Printf("redis disabled: %v", err)
	} else {
		redisCache = c
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Disposition"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	app.RegisterRoutes(r, s, redisCache)

	port := fallbackEnv("PORT", "8080")
	log.Printf(
		"server running at :%s with DB_DRIVER=%s redis=%t",
		port,
		driver,
		redisCache != nil && redisCache.Enabled(),
	)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server exit: %v", err)
	}
}

func fallbackEnv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func fallbackIntEnv(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
