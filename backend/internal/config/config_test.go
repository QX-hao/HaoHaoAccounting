package config

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestLoadDotEnvSetsMissingValuesOnly(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(`
DB_DRIVER=mysql
DB_DSN="dsn-from-file"
REDIS_DB=3
`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}

	if got := os.Getenv("DB_DRIVER"); got != "postgres" {
		t.Fatalf("DB_DRIVER = %q", got)
	}
	if got := os.Getenv("DB_DSN"); got != "dsn-from-file" {
		t.Fatalf("DB_DSN = %q", got)
	}
	if got := os.Getenv("REDIS_DB"); got != "3" {
		t.Fatalf("REDIS_DB = %q", got)
	}
}

func TestLoadDotEnvDoesNotOverrideExplicitEmptyValues(t *testing.T) {
	t.Setenv("LOAD_DOTENV_TEST_EMPTY", "")
	restoreEnv(t, "LOAD_DOTENV_TEST_MISSING")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("LOAD_DOTENV_TEST_EMPTY=secret-from-file\nLOAD_DOTENV_TEST_MISSING=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}

	if got := os.Getenv("LOAD_DOTENV_TEST_EMPTY"); got != "" {
		t.Fatalf("LOAD_DOTENV_TEST_EMPTY = %q, want explicit empty value preserved", got)
	}
	if got := os.Getenv("LOAD_DOTENV_TEST_MISSING"); got != "from-file" {
		t.Fatalf("LOAD_DOTENV_TEST_MISSING = %q", got)
	}
}

func TestLoadDotEnvParsesCommonDotEnvSyntax(t *testing.T) {
	keys := []string{
		"LOAD_DOTENV_USERNAME",
		"LOAD_DOTENV_PASSWORD",
		"LOAD_DOTENV_NAME",
		"LOAD_DOTENV_DSN",
		"LOAD_DOTENV_ISSUER",
		"LOAD_DOTENV_AUDIENCE",
		"exported",
	}
	for _, key := range keys {
		restoreEnv(t, key)
	}
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("\ufeff# local development\nexport LOAD_DOTENV_USERNAME=admin # owner login\nexport\tLOAD_DOTENV_PASSWORD=\"pass#word\"\nLOAD_DOTENV_NAME='好好 # 用户'\nLOAD_DOTENV_DSN=postgres://localhost/db#fragment\nLOAD_DOTENV_ISSUER=\"line\\nissuer\"\nLOAD_DOTENV_AUDIENCE=\"haohao\\\\api\"\nexported=value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := LoadDotEnv(path); err != nil {
		t.Fatalf("LoadDotEnv: %v", err)
	}

	if got := os.Getenv("LOAD_DOTENV_USERNAME"); got != "admin" {
		t.Fatalf("LOAD_DOTENV_USERNAME = %q", got)
	}
	if got := os.Getenv("LOAD_DOTENV_PASSWORD"); got != "pass#word" {
		t.Fatalf("LOAD_DOTENV_PASSWORD = %q", got)
	}
	if got := os.Getenv("LOAD_DOTENV_NAME"); got != "好好 # 用户" {
		t.Fatalf("LOAD_DOTENV_NAME = %q", got)
	}
	if got := os.Getenv("LOAD_DOTENV_DSN"); got != "postgres://localhost/db#fragment" {
		t.Fatalf("LOAD_DOTENV_DSN = %q", got)
	}
	if got := os.Getenv("LOAD_DOTENV_ISSUER"); got != "line\nissuer" {
		t.Fatalf("LOAD_DOTENV_ISSUER = %q", got)
	}
	if got := os.Getenv("LOAD_DOTENV_AUDIENCE"); got != "haohao\\api" {
		t.Fatalf("LOAD_DOTENV_AUDIENCE = %q", got)
	}
	if got := os.Getenv("exported"); got != "value" {
		t.Fatalf("exported = %q", got)
	}
}

func TestLoadDotEnvIgnoresMissingFile(t *testing.T) {
	if err := LoadDotEnv(filepath.Join(t.TempDir(), "missing.env")); err != nil {
		t.Fatalf("LoadDotEnv missing file: %v", err)
	}
}

func TestLoadDefaultsForLocalDevelopment(t *testing.T) {
	clearConfigEnv(t)

	cfg := Load()

	if cfg.Port != "8080" {
		t.Fatalf("Port = %q", cfg.Port)
	}
	if cfg.Database.Driver != "postgres" {
		t.Fatalf("Database.Driver = %q", cfg.Database.Driver)
	}
	if cfg.Database.DSN != defaultPostgresDSN {
		t.Fatalf("Database.DSN = %q", cfg.Database.DSN)
	}
	if cfg.Database.MaxOpenConns != defaultDBMaxOpenConns ||
		cfg.Database.MaxIdleConns != defaultDBMaxIdleConns ||
		cfg.Database.ConnMaxLifetime != defaultDBConnMaxLifetime ||
		cfg.Database.ConnMaxIdleTime != defaultDBConnMaxIdleTime {
		t.Fatalf("Database pool config = %#v", cfg.Database)
	}
	if cfg.Redis.Addr != "127.0.0.1:56379" || cfg.Redis.Password != "haohao123" || cfg.Redis.DB != 0 {
		t.Fatalf("Redis = %#v", cfg.Redis)
	}
	if cfg.HTTP.GinMode != "release" {
		t.Fatalf("HTTP.GinMode = %q", cfg.HTTP.GinMode)
	}
	wantOrigins := []string{"http://localhost:3000", "http://127.0.0.1:3000"}
	if !reflect.DeepEqual(cfg.HTTP.CORSAllowOrigins, wantOrigins) {
		t.Fatalf("CORSAllowOrigins = %#v", cfg.HTTP.CORSAllowOrigins)
	}
	if cfg.Admin.Name != "好好用户" {
		t.Fatalf("Admin.Name = %q", cfg.Admin.Name)
	}
	if cfg.HTTP.ReadTimeout != 15*time.Second ||
		cfg.HTTP.ReadHeaderTimeout != 5*time.Second ||
		cfg.HTTP.WriteTimeout != 30*time.Second ||
		cfg.HTTP.IdleTimeout != 60*time.Second ||
		cfg.HTTP.ShutdownTimeout != 10*time.Second ||
		cfg.HTTP.RequestTimeout != defaultHTTPRequestTimeout ||
		cfg.HTTP.MaxHeaderBytes != 1<<20 ||
		cfg.HTTP.MaxBodyBytes != 6*1024*1024 ||
		cfg.HTTP.MetricsEnabled ||
		cfg.HTTP.MetricsToken != "" ||
		cfg.HTTP.HSTSMaxAgeSeconds != 0 ||
		cfg.HTTP.HSTSIncludeSubDomains ||
		cfg.HTTP.HSTSPreload ||
		cfg.HTTP.CrossOriginEmbedderPolicy != "" {
		t.Fatalf("HTTP config = %#v", cfg.HTTP)
	}
	if cfg.LoginRateLimit.MaxFailures != 5 || cfg.LoginRateLimit.Window != 10*time.Minute {
		t.Fatalf("LoginRateLimit = %#v", cfg.LoginRateLimit)
	}
	if cfg.JWT.Secret != "" {
		t.Fatalf("JWT.Secret = %q", cfg.JWT.Secret)
	}
	if cfg.JWT.TTL != 7*24*time.Hour {
		t.Fatalf("JWT.TTL = %s", cfg.JWT.TTL)
	}
	if cfg.JWT.ClockSkew != 30*time.Second {
		t.Fatalf("JWT.ClockSkew = %s", cfg.JWT.ClockSkew)
	}
	if cfg.JWT.Issuer != "haohao-accounting" || cfg.JWT.Audience != "haohao-accounting-api" {
		t.Fatalf("JWT = %#v", cfg.JWT)
	}
}

func TestLoadParsesEnvironmentOverrides(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("PORT", "9000")
	t.Setenv("DB_DRIVER", "mysql")
	t.Setenv("DB_MAX_OPEN_CONNS", "40")
	t.Setenv("DB_MAX_IDLE_CONNS", "12")
	t.Setenv("DB_CONN_MAX_LIFETIME", "45m")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "5m")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("REDIS_PASSWORD", "secret")
	t.Setenv("REDIS_DB", "2")
	t.Setenv("GIN_MODE", "debug")
	t.Setenv("CORS_ALLOW_ORIGINS", " https://app.example.com,https://admin.example.com ,, ")
	t.Setenv("TRUSTED_PROXIES", " 10.0.0.0/8,192.168.0.0/16 ")
	t.Setenv("HTTP_READ_TIMEOUT", "11s")
	t.Setenv("HTTP_READ_HEADER_TIMEOUT", "3s")
	t.Setenv("HTTP_WRITE_TIMEOUT", "22s")
	t.Setenv("HTTP_IDLE_TIMEOUT", "44s")
	t.Setenv("HTTP_SHUTDOWN_TIMEOUT", "7s")
	t.Setenv("HTTP_REQUEST_TIMEOUT", "9s")
	t.Setenv("HTTP_MAX_HEADER_BYTES", "32768")
	t.Setenv("HTTP_MAX_BODY_BYTES", "123456")
	t.Setenv("HTTP_METRICS_ENABLED", "true")
	t.Setenv("HTTP_METRICS_TOKEN", " scrape-secret ")
	t.Setenv("HTTP_HSTS_MAX_AGE_SECONDS", "31536000")
	t.Setenv("HTTP_HSTS_INCLUDE_SUBDOMAINS", "true")
	t.Setenv("HTTP_HSTS_PRELOAD", "true")
	t.Setenv("HTTP_CROSS_ORIGIN_EMBEDDER_POLICY", "require-corp")
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "password")
	t.Setenv("ADMIN_NAME", "Owner")
	t.Setenv("LOGIN_RATE_LIMIT_MAX_FAILURES", "3")
	t.Setenv("LOGIN_RATE_LIMIT_WINDOW", "2m")
	t.Setenv("JWT_SECRET", "jwt-secret-with-at-least-32-characters")
	t.Setenv("JWT_TTL", "30m")
	t.Setenv("JWT_CLOCK_SKEW", "45s")
	t.Setenv("JWT_ISSUER", "https://api.example.com")
	t.Setenv("JWT_AUDIENCE", "haohao-web")

	cfg := Load()

	if cfg.Port != "9000" {
		t.Fatalf("Port = %q", cfg.Port)
	}
	if cfg.Database.Driver != "mysql" {
		t.Fatalf("Database.Driver = %q", cfg.Database.Driver)
	}
	if cfg.Database.DSN != defaultMySQLDSN {
		t.Fatalf("Database.DSN = %q", cfg.Database.DSN)
	}
	if cfg.Database.MaxOpenConns != 40 ||
		cfg.Database.MaxIdleConns != 12 ||
		cfg.Database.ConnMaxLifetime != 45*time.Minute ||
		cfg.Database.ConnMaxIdleTime != 5*time.Minute {
		t.Fatalf("Database pool config = %#v", cfg.Database)
	}
	if cfg.Redis.Addr != "redis:6379" || cfg.Redis.Password != "secret" || cfg.Redis.DB != 2 {
		t.Fatalf("Redis = %#v", cfg.Redis)
	}
	if cfg.HTTP.GinMode != "debug" {
		t.Fatalf("HTTP.GinMode = %q", cfg.HTTP.GinMode)
	}
	if want := []string{"https://app.example.com", "https://admin.example.com"}; !reflect.DeepEqual(cfg.HTTP.CORSAllowOrigins, want) {
		t.Fatalf("CORSAllowOrigins = %#v", cfg.HTTP.CORSAllowOrigins)
	}
	if want := []string{"10.0.0.0/8", "192.168.0.0/16"}; !reflect.DeepEqual(cfg.HTTP.TrustedProxies, want) {
		t.Fatalf("TrustedProxies = %#v", cfg.HTTP.TrustedProxies)
	}
	if cfg.HTTP.ReadTimeout != 11*time.Second ||
		cfg.HTTP.ReadHeaderTimeout != 3*time.Second ||
		cfg.HTTP.WriteTimeout != 22*time.Second ||
		cfg.HTTP.IdleTimeout != 44*time.Second ||
		cfg.HTTP.ShutdownTimeout != 7*time.Second ||
		cfg.HTTP.RequestTimeout != 9*time.Second ||
		cfg.HTTP.MaxHeaderBytes != 32768 ||
		cfg.HTTP.MaxBodyBytes != 123456 ||
		!cfg.HTTP.MetricsEnabled ||
		cfg.HTTP.MetricsToken != "scrape-secret" ||
		cfg.HTTP.HSTSMaxAgeSeconds != 31536000 ||
		!cfg.HTTP.HSTSIncludeSubDomains ||
		!cfg.HTTP.HSTSPreload ||
		cfg.HTTP.CrossOriginEmbedderPolicy != "require-corp" {
		t.Fatalf("HTTP config = %#v", cfg.HTTP)
	}
	if cfg.Admin.Username != "admin" || cfg.Admin.Password != "password" || cfg.Admin.Name != "Owner" {
		t.Fatalf("Admin = %#v", cfg.Admin)
	}
	if cfg.LoginRateLimit.MaxFailures != 3 || cfg.LoginRateLimit.Window != 2*time.Minute {
		t.Fatalf("LoginRateLimit = %#v", cfg.LoginRateLimit)
	}
	if cfg.JWT.Secret != "jwt-secret-with-at-least-32-characters" {
		t.Fatalf("JWT.Secret = %q", cfg.JWT.Secret)
	}
	if cfg.JWT.TTL != 30*time.Minute {
		t.Fatalf("JWT.TTL = %s", cfg.JWT.TTL)
	}
	if cfg.JWT.ClockSkew != 45*time.Second {
		t.Fatalf("JWT.ClockSkew = %s", cfg.JWT.ClockSkew)
	}
	if cfg.JWT.Issuer != "https://api.example.com" || cfg.JWT.Audience != "haohao-web" {
		t.Fatalf("JWT = %#v", cfg.JWT)
	}
}

func TestLoadStrictRejectsInvalidEnvironmentValues(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "password")
	t.Setenv("JWT_SECRET", "jwt-secret-with-at-least-32-characters")
	t.Setenv("PORT", "70000")
	t.Setenv("DB_MAX_OPEN_CONNS", "0")
	t.Setenv("HTTP_READ_TIMEOUT", "not-a-duration")
	t.Setenv("HTTP_REQUEST_TIMEOUT", "-1s")
	t.Setenv("HTTP_METRICS_ENABLED", "not-a-bool")
	t.Setenv("HTTP_METRICS_TOKEN", "contains space")
	t.Setenv("HTTP_HSTS_INCLUDE_SUBDOMAINS", "not-a-bool")
	t.Setenv("HTTP_CROSS_ORIGIN_EMBEDDER_POLICY", "same-origin")
	t.Setenv("JWT_TTL", "0s")

	_, err := LoadStrict()
	if err == nil {
		t.Fatal("expected strict config error")
	}
	message := err.Error()
	for _, want := range []string{
		"PORT",
		"DB_MAX_OPEN_CONNS",
		"HTTP_READ_TIMEOUT",
		"HTTP_REQUEST_TIMEOUT",
		"HTTP_METRICS_ENABLED",
		"HTTP_METRICS_TOKEN",
		"HTTP_HSTS_INCLUDE_SUBDOMAINS",
		"HTTP_CROSS_ORIGIN_EMBEDDER_POLICY",
		"JWT_TTL",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("strict error %q does not mention %s", message, want)
		}
	}
}

func TestLoadStrictValidatesLoadedConfig(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "password")
	t.Setenv("JWT_SECRET", "jwt-secret-with-at-least-32-characters")
	t.Setenv("JWT_TTL", "1h")

	cfg, err := LoadStrict()
	if err != nil {
		t.Fatalf("LoadStrict: %v", err)
	}
	if cfg.Admin.Username != "admin" || cfg.JWT.TTL != time.Hour {
		t.Fatalf("Config = %#v", cfg)
	}
}

func TestLoadStrictAllowsDisabledLoginRateLimiter(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "password")
	t.Setenv("JWT_SECRET", "jwt-secret-with-at-least-32-characters")
	t.Setenv("LOGIN_RATE_LIMIT_MAX_FAILURES", "0")
	t.Setenv("LOGIN_RATE_LIMIT_WINDOW", "0s")

	cfg, err := LoadStrict()
	if err != nil {
		t.Fatalf("LoadStrict disabled login limiter: %v", err)
	}
	if cfg.LoginRateLimit.MaxFailures != 0 || cfg.LoginRateLimit.Window != 0 {
		t.Fatalf("LoginRateLimit = %#v", cfg.LoginRateLimit)
	}
}

func TestJWTConfigValidate(t *testing.T) {
	if err := (JWTConfig{}).Validate(); err == nil {
		t.Fatal("expected missing secret error")
	}
	if err := (JWTConfig{Secret: "short"}).Validate(); err == nil {
		t.Fatal("expected short secret error")
	}
	if err := (JWTConfig{Secret: "jwt-secret-with-at-least-32-characters"}).Validate(); err == nil {
		t.Fatal("expected ttl error")
	}
	if err := (JWTConfig{Secret: "jwt-secret-with-at-least-32-characters", TTL: time.Hour, ClockSkew: -time.Second, Issuer: "issuer", Audience: "api"}).Validate(); err == nil {
		t.Fatal("expected clock skew error")
	}
	if err := (JWTConfig{Secret: "jwt-secret-with-at-least-32-characters", TTL: time.Hour, Audience: "api"}).Validate(); err == nil {
		t.Fatal("expected issuer error")
	}
	if err := (JWTConfig{Secret: "jwt-secret-with-at-least-32-characters", TTL: time.Hour, Issuer: "issuer"}).Validate(); err == nil {
		t.Fatal("expected audience error")
	}
	if err := (JWTConfig{Secret: "jwt-secret-with-at-least-32-characters", TTL: time.Hour, Issuer: "issuer", Audience: "api"}).Validate(); err != nil {
		t.Fatalf("valid secret error: %v", err)
	}
}

func TestDatabaseConfigValidate(t *testing.T) {
	valid := validDatabaseConfig()
	for _, driver := range []string{"postgres", "pgsql", "mysql", " POSTGRES "} {
		t.Run(driver, func(t *testing.T) {
			cfg := valid
			cfg.Driver = driver
			if err := cfg.Validate(); err != nil {
				t.Fatalf("valid driver error: %v", err)
			}
		})
	}
	invalidDriver := valid
	invalidDriver.Driver = "sqlite"
	if err := invalidDriver.Validate(); err == nil {
		t.Fatal("expected unsupported driver error")
	}
	invalidPool := valid
	invalidPool.MaxOpenConns = 0
	if err := invalidPool.Validate(); err == nil {
		t.Fatal("expected invalid pool config error")
	}
	invalidIdleLimit := valid
	invalidIdleLimit.MaxOpenConns = 5
	invalidIdleLimit.MaxIdleConns = 6
	if err := invalidIdleLimit.Validate(); err == nil {
		t.Fatal("expected idle connections above open connections error")
	}
}

func TestHTTPConfigValidate(t *testing.T) {
	valid := validHTTPConfig()
	for _, mode := range []string{"debug", "release", "test", " RELEASE "} {
		t.Run(mode, func(t *testing.T) {
			cfg := valid
			cfg.GinMode = mode
			if err := cfg.Validate(); err != nil {
				t.Fatalf("valid mode error: %v", err)
			}
		})
	}
	invalidMode := valid
	invalidMode.GinMode = "production"
	if err := invalidMode.Validate(); err == nil {
		t.Fatal("expected unsupported gin mode error")
	}
	invalidReadTimeout := valid
	invalidReadTimeout.ReadTimeout = 0
	if err := invalidReadTimeout.Validate(); err == nil {
		t.Fatal("expected non-positive read timeout error")
	}
	invalidBodyLimit := valid
	invalidBodyLimit.MaxBodyBytes = 0
	if err := invalidBodyLimit.Validate(); err == nil {
		t.Fatal("expected non-positive body limit error")
	}
	invalidHSTSMaxAge := valid
	invalidHSTSMaxAge.HSTSMaxAgeSeconds = -1
	if err := invalidHSTSMaxAge.Validate(); err == nil {
		t.Fatal("expected negative hsts max-age error")
	}
	invalidRequestTimeout := valid
	invalidRequestTimeout.RequestTimeout = -time.Second
	if err := invalidRequestTimeout.Validate(); err == nil {
		t.Fatal("expected negative request timeout error")
	}
	invalidMetricsToken := valid
	invalidMetricsToken.MetricsToken = "contains space"
	if err := invalidMetricsToken.Validate(); err == nil {
		t.Fatal("expected invalid metrics token error")
	}
	paddingOnlyMetricsToken := valid
	paddingOnlyMetricsToken.MetricsToken = "=="
	if err := paddingOnlyMetricsToken.Validate(); err == nil {
		t.Fatal("expected padding-only metrics token error")
	}
	tooLongMetricsToken := valid
	tooLongMetricsToken.MetricsToken = strings.Repeat("a", 4097)
	if err := tooLongMetricsToken.Validate(); err == nil {
		t.Fatal("expected overlong metrics token error")
	}
	for _, policy := range []string{"require-corp", "credentialless", "unsafe-none", " REQUIRE-CORP "} {
		t.Run("valid COEP "+policy, func(t *testing.T) {
			cfg := valid
			cfg.CrossOriginEmbedderPolicy = policy
			if err := cfg.Validate(); err != nil {
				t.Fatalf("valid cross-origin embedder policy error: %v", err)
			}
		})
	}
	invalidCOEP := valid
	invalidCOEP.CrossOriginEmbedderPolicy = "same-origin"
	if err := invalidCOEP.Validate(); err == nil {
		t.Fatal("expected invalid cross-origin embedder policy error")
	}
	invalidHSTSDirective := valid
	invalidHSTSDirective.HSTSIncludeSubDomains = true
	if err := invalidHSTSDirective.Validate(); err == nil {
		t.Fatal("expected hsts directive without max-age error")
	}
	invalidHSTSPreload := valid
	invalidHSTSPreload.HSTSPreload = true
	if err := invalidHSTSPreload.Validate(); err == nil {
		t.Fatal("expected hsts preload without preload max-age error")
	}
	invalidHSTSPreloadMaxAge := valid
	invalidHSTSPreloadMaxAge.HSTSMaxAgeSeconds = 300
	invalidHSTSPreloadMaxAge.HSTSIncludeSubDomains = true
	invalidHSTSPreloadMaxAge.HSTSPreload = true
	if err := invalidHSTSPreloadMaxAge.Validate(); err == nil {
		t.Fatal("expected hsts preload below preload max-age error")
	}
	invalidHSTSPreloadSubdomains := valid
	invalidHSTSPreloadSubdomains.HSTSMaxAgeSeconds = 31536000
	invalidHSTSPreloadSubdomains.HSTSPreload = true
	if err := invalidHSTSPreloadSubdomains.Validate(); err == nil {
		t.Fatal("expected hsts preload without includeSubDomains error")
	}
	validHSTS := valid
	validHSTS.HSTSMaxAgeSeconds = 31536000
	validHSTS.HSTSIncludeSubDomains = true
	validHSTS.HSTSPreload = true
	if err := validHSTS.Validate(); err != nil {
		t.Fatalf("valid hsts config error: %v", err)
	}
}

func TestHTTPConfigValidateTrustedProxies(t *testing.T) {
	valid := HTTPConfig{
		GinMode:           "release",
		ReadTimeout:       time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      time.Second,
		IdleTimeout:       time.Second,
		ShutdownTimeout:   time.Second,
		MaxHeaderBytes:    1,
		MaxBodyBytes:      1,
		TrustedProxies: []string{
			"127.0.0.1",
			"10.0.0.0/8",
			"::1",
			"fd00::/8",
		},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid trusted proxies error: %v", err)
	}

	tests := []struct {
		name    string
		proxies []string
	}{
		{name: "wildcard ipv4 cidr", proxies: []string{"0.0.0.0/0"}},
		{name: "wildcard ipv6 cidr", proxies: []string{"::/0"}},
		{name: "unspecified ipv4", proxies: []string{"0.0.0.0"}},
		{name: "unspecified ipv6", proxies: []string{"::"}},
		{name: "unspecified ipv4 cidr", proxies: []string{"0.0.0.0/8"}},
		{name: "hostname", proxies: []string{"proxy.internal"}},
		{name: "invalid cidr", proxies: []string{"10.0.0.0/33"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid
			cfg.TrustedProxies = tt.proxies
			if err := cfg.Validate(); err == nil {
				t.Fatalf("expected trusted proxies error for %#v", tt.proxies)
			}
		})
	}
}

func TestLoadFallsBackWhenHSTSValuesAreInvalid(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("HTTP_HSTS_MAX_AGE_SECONDS", "-1")
	t.Setenv("HTTP_HSTS_INCLUDE_SUBDOMAINS", "not-a-bool")
	t.Setenv("HTTP_HSTS_PRELOAD", "not-a-bool")

	cfg := Load()

	if cfg.HTTP.HSTSMaxAgeSeconds != 0 || cfg.HTTP.HSTSIncludeSubDomains || cfg.HTTP.HSTSPreload {
		t.Fatalf("HSTS config = %#v", cfg.HTTP)
	}
}

func TestAdminConfigValidate(t *testing.T) {
	if err := (AdminConfig{}).Validate(); err == nil {
		t.Fatal("expected missing username error")
	}
	if err := (AdminConfig{Username: "admin"}).Validate(); err == nil {
		t.Fatal("expected missing password error")
	}
	if err := (AdminConfig{Username: " admin ", Password: " secret "}).Validate(); err != nil {
		t.Fatalf("valid admin config error: %v", err)
	}
}

func TestRedisConfigValidate(t *testing.T) {
	if err := (RedisConfig{Addr: "127.0.0.1:6379", DB: 0}).Validate(); err != nil {
		t.Fatalf("valid redis config error: %v", err)
	}
	if err := (RedisConfig{Addr: "localhost:6379", DB: 0}).Validate(); err != nil {
		t.Fatalf("valid local redis without password error: %v", err)
	}
	if err := (RedisConfig{Addr: "[::1]:6379", DB: 0}).Validate(); err != nil {
		t.Fatalf("valid ipv6 loopback redis without password error: %v", err)
	}
	if err := (RedisConfig{Addr: "redis:6379", Password: "secret", DB: 0}).Validate(); err != nil {
		t.Fatalf("valid remote redis with password error: %v", err)
	}
	if err := (RedisConfig{Addr: "redis:6379", DB: -1}).Validate(); err == nil {
		t.Fatal("expected negative redis db error")
	}
	if err := (RedisConfig{Addr: "", Password: "secret", DB: 0}).Validate(); err == nil {
		t.Fatal("expected missing redis addr error")
	}
	if err := (RedisConfig{Addr: "redis", Password: "secret", DB: 0}).Validate(); err == nil {
		t.Fatal("expected redis addr host:port error")
	}
	if err := (RedisConfig{Addr: "redis:6379", DB: 0}).Validate(); err == nil {
		t.Fatal("expected remote redis password error")
	}
}

func TestLoginRateLimitConfigValidate(t *testing.T) {
	if err := (LoginRateLimitConfig{MaxFailures: 1, Window: time.Minute}).Validate(); err != nil {
		t.Fatalf("valid login rate limit error: %v", err)
	}
	if err := (LoginRateLimitConfig{MaxFailures: 0, Window: time.Minute}).Validate(); err != nil {
		t.Fatalf("zero max failures should disable limiter: %v", err)
	}
	if err := (LoginRateLimitConfig{MaxFailures: 1, Window: 0}).Validate(); err != nil {
		t.Fatalf("zero window should disable limiter: %v", err)
	}
	if err := (LoginRateLimitConfig{MaxFailures: -1, Window: time.Minute}).Validate(); err == nil {
		t.Fatal("expected invalid max failures error")
	}
	if err := (LoginRateLimitConfig{MaxFailures: 1, Window: -time.Second}).Validate(); err == nil {
		t.Fatal("expected invalid window error")
	}
}

func TestConfigValidate(t *testing.T) {
	valid := Config{
		Port:           "8080",
		Database:       validDatabaseConfig(),
		Redis:          RedisConfig{Addr: "127.0.0.1:6379", DB: 0},
		HTTP:           validHTTPConfig(),
		LoginRateLimit: LoginRateLimitConfig{MaxFailures: 5, Window: time.Minute},
		Admin:          AdminConfig{Username: "admin", Password: "secret"},
		JWT:            JWTConfig{Secret: "jwt-secret-with-at-least-32-characters", TTL: time.Hour, Issuer: "issuer", Audience: "api"},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config error: %v", err)
	}

	invalidPort := valid
	invalidPort.Port = "70000"
	if err := invalidPort.Validate(); err == nil {
		t.Fatal("expected invalid port error")
	}

	invalidDriver := valid
	invalidDriver.Database.Driver = "sqlite"
	if err := invalidDriver.Validate(); err == nil {
		t.Fatal("expected invalid database driver error")
	}

	invalidMode := valid
	invalidMode.HTTP.GinMode = "production"
	if err := invalidMode.Validate(); err == nil {
		t.Fatal("expected invalid gin mode error")
	}

	missingJWT := valid
	missingJWT.JWT.Secret = ""
	if err := missingJWT.Validate(); err == nil {
		t.Fatal("expected missing jwt secret error")
	}

	missingAdmin := valid
	missingAdmin.Admin.Password = ""
	if err := missingAdmin.Validate(); err == nil {
		t.Fatal("expected missing admin password error")
	}

	multipleInvalid := valid
	multipleInvalid.Database.MaxOpenConns = 0
	multipleInvalid.HTTP.MaxBodyBytes = 0
	multipleInvalid.LoginRateLimit.Window = -time.Second
	if err := multipleInvalid.Validate(); err == nil {
		t.Fatal("expected multiple validation errors")
	} else {
		message := err.Error()
		for _, want := range []string{"DB_MAX_OPEN_CONNS", "HTTP_MAX_BODY_BYTES", "LOGIN_RATE_LIMIT_WINDOW"} {
			if !strings.Contains(message, want) {
				t.Fatalf("validation error %q does not mention %s", message, want)
			}
		}
	}
}

func TestConfigValidateRejectsDevelopmentSecretsForPublicOrigins(t *testing.T) {
	valid := Config{
		Port:           "8080",
		Database:       validDatabaseConfig(),
		Redis:          RedisConfig{Addr: "redis:6379", Password: "redis-secret", DB: 0},
		HTTP:           validHTTPConfig(),
		LoginRateLimit: LoginRateLimitConfig{MaxFailures: 5, Window: time.Minute},
		Admin:          AdminConfig{Username: "admin", Password: "admin-secret"},
		JWT:            JWTConfig{Secret: "jwt-secret-with-at-least-32-characters", TTL: time.Hour, Issuer: "issuer", Audience: "api"},
	}
	valid.HTTP.CORSAllowOrigins = []string{"https://app.example.com"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid public config error: %v", err)
	}

	insecure := valid
	insecure.Redis.Password = developmentRedisPassword
	insecure.Admin.Password = developmentAdminPassword
	insecure.JWT.Secret = developmentJWTSecret
	if err := insecure.Validate(); err == nil {
		t.Fatal("expected development secret error for public origin")
	} else {
		message := err.Error()
		for _, want := range []string{"JWT_SECRET", "ADMIN_PASSWORD", "REDIS_PASSWORD"} {
			if !strings.Contains(message, want) {
				t.Fatalf("validation error %q does not mention %s", message, want)
			}
		}
	}
}

func TestConfigValidateAllowsDevelopmentSecretsForLocalOrigins(t *testing.T) {
	cfg := Config{
		Port:           "8080",
		Database:       validDatabaseConfig(),
		Redis:          RedisConfig{Addr: "127.0.0.1:6379", Password: developmentRedisPassword, DB: 0},
		HTTP:           validHTTPConfig(),
		LoginRateLimit: LoginRateLimitConfig{MaxFailures: 5, Window: time.Minute},
		Admin:          AdminConfig{Username: "admin", Password: developmentAdminPassword},
		JWT:            JWTConfig{Secret: developmentJWTSecret, TTL: time.Hour, Issuer: "issuer", Audience: "api"},
	}
	cfg.HTTP.CORSAllowOrigins = []string{"http://localhost:3000", "http://127.0.0.1:3000"}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("local development config error: %v", err)
	}
}

func TestConfigEnvironmentKeysStayDocumented(t *testing.T) {
	sourceKeys := configSourceEnvKeys(t)
	exampleKeys := envExampleKeys(t)

	if missing := missingKeys(sourceKeys, exampleKeys); len(missing) > 0 {
		t.Fatalf("backend/.env.example is missing config keys: %v", missing)
	}
	if unexpected := unexpectedKeys(sourceKeys, exampleKeys); len(unexpected) > 0 {
		t.Fatalf("backend/.env.example documents unknown config keys: %v", unexpected)
	}
}

func TestEnvExampleDocumentsExplicitCORSOrigins(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatalf("read backend/.env.example: %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"explicit HTTP(S) origins only",
		"Do not include paths, query",
		"fragments, or wildcards",
		"CORS_ALLOW_ORIGINS=http://localhost:3000,http://127.0.0.1:3000",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("backend/.env.example is missing CORS guidance %q", want)
		}
	}
}

func TestEnvExampleDocumentsDefaultRequestTimeout(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatalf("read backend/.env.example: %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"Per-request context deadline",
		"Use 0s to disable",
		"HTTP_REQUEST_TIMEOUT=60s",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("backend/.env.example is missing request timeout guidance %q", want)
		}
	}
}

func TestEnvExampleDocumentsMetricsExposure(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatalf("read backend/.env.example: %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"Exposes /metrics on the backend port",
		"Keep disabled unless the port is protected",
		"HTTP_METRICS_ENABLED=false",
		"HTTP_METRICS_TOKEN=",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("backend/.env.example is missing metrics guidance %q", want)
		}
	}
}

func TestEnvExampleDocumentsCrossOriginEmbedderPolicy(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatalf("read backend/.env.example: %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"Optional COEP header",
		"Allowed values: require-corp, credentialless, unsafe-none",
		"HTTP_CROSS_ORIGIN_EMBEDDER_POLICY=",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("backend/.env.example is missing COEP guidance %q", want)
		}
	}
}

func TestEnvExampleDocumentsLoginRateLimitDisableValues(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatalf("read backend/.env.example: %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"Login rate limiting is enabled when both values are positive",
		"Set either value",
		"0s only for trusted local debugging",
		"LOGIN_RATE_LIMIT_MAX_FAILURES=5",
		"LOGIN_RATE_LIMIT_WINDOW=10m",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("backend/.env.example is missing login rate limit guidance %q", want)
		}
	}
}

func TestEnvExampleLoadsStrictly(t *testing.T) {
	unsetConfigEnv(t)

	if err := LoadDotEnv(filepath.Join("..", "..", ".env.example")); err != nil {
		t.Fatalf("LoadDotEnv backend/.env.example: %v", err)
	}
	cfg, err := LoadStrict()
	if err != nil {
		t.Fatalf("LoadStrict with backend/.env.example: %v", err)
	}
	if cfg.JWT.Secret == "" {
		t.Fatal("backend/.env.example must provide a development JWT_SECRET")
	}
	if cfg.Admin.Password == "" {
		t.Fatal("backend/.env.example must provide a development ADMIN_PASSWORD")
	}
}

func TestComposeBackendEnvironmentCoversConfigKeys(t *testing.T) {
	sourceKeys := configSourceEnvKeys(t)

	for _, service := range []string{"backend", "dbmigrate"} {
		t.Run(service, func(t *testing.T) {
			composeKeys := composeServiceEnvironmentKeys(t, service)
			if missing := missingKeys(sourceKeys, composeKeys); len(missing) > 0 {
				t.Fatalf("docker-compose.yaml %s environment is missing config keys: %v", service, missing)
			}
			if unexpected := unexpectedKeys(sourceKeys, composeKeys); len(unexpected) > 0 {
				t.Fatalf("docker-compose.yaml %s environment has unknown config keys: %v", service, unexpected)
			}
		})
	}
}

func TestProductionComposeReusesBackendEnvironmentAnchor(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read docker-compose.yaml: %v", err)
	}
	source := string(data)

	if !strings.Contains(source, "x-backend-env: &backend-env") {
		t.Fatal("docker-compose.yaml must define x-backend-env anchor")
	}
	for _, service := range []string{"backend", "dbmigrate"} {
		block := composeServiceBlock(source, service)
		if !strings.Contains(block, "environment: *backend-env") {
			t.Fatalf("%s service must reuse backend environment anchor:\n%s", service, block)
		}
	}
}

func TestProductionComposeDefaultsRequestTimeout(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read docker-compose.yaml: %v", err)
	}
	source := string(data)

	if !strings.Contains(source, "HTTP_REQUEST_TIMEOUT: ${HTTP_REQUEST_TIMEOUT:-60s}") {
		t.Fatal("docker-compose.yaml must default HTTP_REQUEST_TIMEOUT to 60s")
	}
}

func TestProductionComposeDisablesMetricsByDefault(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read docker-compose.yaml: %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"HTTP_METRICS_ENABLED: ${HTTP_METRICS_ENABLED:-false}",
		"HTTP_METRICS_TOKEN: ${HTTP_METRICS_TOKEN:-}",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("docker-compose.yaml must include metrics default %q", want)
		}
	}
}

func TestProductionComposeDisablesCrossOriginEmbedderPolicyByDefault(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read docker-compose.yaml: %v", err)
	}
	source := string(data)

	if !strings.Contains(source, "HTTP_CROSS_ORIGIN_EMBEDDER_POLICY: ${HTTP_CROSS_ORIGIN_EMBEDDER_POLICY:-}") {
		t.Fatal("docker-compose.yaml must default HTTP_CROSS_ORIGIN_EMBEDDER_POLICY to empty")
	}
}

func TestProductionComposeDocumentsMetricsToken(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "readme.md"))
	if err != nil {
		t.Fatalf("read readme.md: %v", err)
	}
	source := string(data)

	for _, want := range []string{
		"HTTP_METRICS_TOKEN",
		"Authorization: Bearer",
		"RFC 6750 token68",
		"最长 4096 字节",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("readme.md is missing metrics token guidance %q", want)
		}
	}
}

func TestProductionComposeRunsMigrationsBeforeBackend(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read docker-compose.yaml: %v", err)
	}

	backend := composeServiceBlock(string(data), "backend")
	if backend == "" {
		t.Fatal("missing backend service")
	}
	for _, want := range []string{
		"dbmigrate:",
		"condition: service_completed_successfully",
	} {
		if !strings.Contains(backend, want) {
			t.Fatalf("backend service is missing migration dependency %s:\n%s", want, backend)
		}
	}

	dbmigrate := composeServiceBlock(string(data), "dbmigrate")
	if dbmigrate == "" {
		t.Fatal("missing dbmigrate service")
	}
	for _, want := range []string{
		"image: haohaoaccounting-backend:latest",
		"restart: \"no\"",
		"entrypoint: [\"/app/dbmigrate\"]",
		"postgres:",
		"condition: service_healthy",
	} {
		if !strings.Contains(dbmigrate, want) {
			t.Fatalf("dbmigrate service is missing %s:\n%s", want, dbmigrate)
		}
	}
}

func TestLocalComposeDatastoresExposeHealthchecks(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docker-compose.local.yaml"))
	if err != nil {
		t.Fatalf("read docker-compose.local.yaml: %v", err)
	}

	for _, service := range []string{"postgres", "redis", "mysql"} {
		t.Run(service, func(t *testing.T) {
			block := composeServiceBlock(string(data), service)
			if block == "" {
				t.Fatalf("missing %s service", service)
			}
			for _, want := range []string{"healthcheck:", "test:", "interval:", "timeout:", "retries:"} {
				if !strings.Contains(block, want) {
					t.Fatalf("%s service is missing %s in healthcheck block:\n%s", service, want, block)
				}
			}
		})
	}
}

func TestLoadKeepsExplicitDatabaseDSN(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DB_DRIVER", "mysql")
	t.Setenv("DB_DSN", "custom-dsn")

	cfg := Load()

	if cfg.Database.DSN != "custom-dsn" {
		t.Fatalf("Database.DSN = %q", cfg.Database.DSN)
	}
}

func TestLoadFallsBackWhenNumericValuesAreInvalid(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DB_MAX_OPEN_CONNS", "-1")
	t.Setenv("DB_MAX_IDLE_CONNS", "not-a-number")
	t.Setenv("DB_CONN_MAX_LIFETIME", "0s")
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "not-a-duration")
	t.Setenv("REDIS_DB", "not-a-number")
	t.Setenv("HTTP_READ_TIMEOUT", "not-a-duration")
	t.Setenv("HTTP_REQUEST_TIMEOUT", "-1s")
	t.Setenv("HTTP_MAX_HEADER_BYTES", "-1")
	t.Setenv("HTTP_MAX_BODY_BYTES", "-1")
	t.Setenv("LOGIN_RATE_LIMIT_MAX_FAILURES", "not-a-number")
	t.Setenv("LOGIN_RATE_LIMIT_WINDOW", "not-a-duration")
	t.Setenv("JWT_TTL", "not-a-duration")

	cfg := Load()

	if cfg.Redis.DB != 0 {
		t.Fatalf("Redis.DB = %d", cfg.Redis.DB)
	}
	if cfg.Database.MaxOpenConns != defaultDBMaxOpenConns ||
		cfg.Database.MaxIdleConns != defaultDBMaxIdleConns ||
		cfg.Database.ConnMaxLifetime != defaultDBConnMaxLifetime ||
		cfg.Database.ConnMaxIdleTime != defaultDBConnMaxIdleTime {
		t.Fatalf("Database pool config = %#v", cfg.Database)
	}
	if cfg.HTTP.ReadTimeout != 15*time.Second {
		t.Fatalf("HTTP.ReadTimeout = %s", cfg.HTTP.ReadTimeout)
	}
	if cfg.HTTP.RequestTimeout != defaultHTTPRequestTimeout {
		t.Fatalf("HTTP.RequestTimeout = %s", cfg.HTTP.RequestTimeout)
	}
	if cfg.HTTP.MaxHeaderBytes != 1<<20 {
		t.Fatalf("HTTP.MaxHeaderBytes = %d", cfg.HTTP.MaxHeaderBytes)
	}
	if cfg.HTTP.MaxBodyBytes != 6*1024*1024 {
		t.Fatalf("HTTP.MaxBodyBytes = %d", cfg.HTTP.MaxBodyBytes)
	}
	if cfg.LoginRateLimit.MaxFailures != 5 || cfg.LoginRateLimit.Window != 10*time.Minute {
		t.Fatalf("LoginRateLimit = %#v", cfg.LoginRateLimit)
	}
	if cfg.JWT.TTL != 7*24*time.Hour {
		t.Fatalf("JWT.TTL = %s", cfg.JWT.TTL)
	}
	if cfg.JWT.ClockSkew != 30*time.Second {
		t.Fatalf("JWT.ClockSkew = %s", cfg.JWT.ClockSkew)
	}
}

func TestLoadAllowsDisablingRequestTimeout(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("HTTP_REQUEST_TIMEOUT", "0s")

	cfg := Load()

	if cfg.HTTP.RequestTimeout != 0 {
		t.Fatalf("HTTP.RequestTimeout = %s, want disabled", cfg.HTTP.RequestTimeout)
	}
}

func validDatabaseConfig() DatabaseConfig {
	return DatabaseConfig{
		Driver:          "postgres",
		MaxOpenConns:    defaultDBMaxOpenConns,
		MaxIdleConns:    defaultDBMaxIdleConns,
		ConnMaxLifetime: defaultDBConnMaxLifetime,
		ConnMaxIdleTime: defaultDBConnMaxIdleTime,
	}
}

func validHTTPConfig() HTTPConfig {
	return HTTPConfig{
		GinMode:           "release",
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ShutdownTimeout:   10 * time.Second,
		RequestTimeout:    defaultHTTPRequestTimeout,
		MaxHeaderBytes:    1 << 20,
		MaxBodyBytes:      6 * 1024 * 1024,
	}
}

func configSourceEnvKeys(t *testing.T) []string {
	t.Helper()

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "config.go", nil, 0)
	if err != nil {
		t.Fatalf("parse config.go: %v", err)
	}

	keys := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		if key, ok := envKeyFromCall(call); ok {
			keys[key] = true
		}
		return true
	})
	return sortedKeys(keys)
}

func envKeyFromCall(call *ast.CallExpr) (string, bool) {
	if len(call.Args) == 0 || !isEnvReader(call.Fun) {
		return "", false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	key := strings.Trim(literal.Value, `"`)
	if !isEnvKey(key) {
		return "", false
	}
	return key, true
}

func isEnvReader(expr ast.Expr) bool {
	if selector, ok := expr.(*ast.SelectorExpr); ok {
		ident, ok := selector.X.(*ast.Ident)
		return ok && ident.Name == "os" && (selector.Sel.Name == "Getenv" || selector.Sel.Name == "LookupEnv")
	}

	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	switch ident.Name {
	case "stringEnv",
		"intEnv",
		"nonNegativeIntEnv",
		"positiveIntEnv",
		"boolEnv",
		"crossOriginEmbedderPolicyEnv",
		"int64Env",
		"durationEnv",
		"nonNegativeDurationEnv",
		"csvEnv",
		"validatePortEnv",
		"validateIntEnv",
		"validateNonNegativeIntEnv",
		"validatePositiveIntEnv",
		"validatePositiveInt64Env",
		"validateBoolEnv",
		"validatePositiveDurationEnv",
		"validateNonNegativeDurationEnv",
		"validateCrossOriginEmbedderPolicyEnv":
		return true
	default:
		return false
	}
}

func envExampleKeys(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatalf("read backend/.env.example: %v", err)
	}

	keys := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "#"))
		key, _, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if isEnvKey(key) {
			keys[key] = true
		}
	}
	return sortedKeys(keys)
}

func composeServiceEnvironmentKeys(t *testing.T, service string) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read docker-compose.yaml: %v", err)
	}

	keys := map[string]bool{}
	source := string(data)
	anchorKeys := composeBackendEnvAnchorKeys(source)
	inBackend, inEnvironment := false, false
	serviceMarker := "  " + service + ":"
	for _, line := range strings.Split(source, "\n") {
		if line == serviceMarker {
			inBackend = true
			inEnvironment = false
			continue
		}
		if inBackend && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && line != serviceMarker {
			break
		}
		if !inBackend {
			continue
		}
		if strings.HasPrefix(line, "    environment:") {
			if strings.Contains(line, "*backend-env") {
				for _, key := range anchorKeys {
					keys[key] = true
				}
				return sortedKeys(keys)
			}
			inEnvironment = true
			continue
		}
		if inEnvironment && strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      ") {
			break
		}
		if !inEnvironment {
			continue
		}
		clean := strings.TrimSpace(line)
		key, _, ok := strings.Cut(clean, ":")
		if ok && isEnvKey(key) {
			keys[key] = true
		}
	}
	return sortedKeys(keys)
}

func composeBackendEnvAnchorKeys(data string) []string {
	keys := map[string]bool{}
	inAnchor := false
	for _, line := range strings.Split(data, "\n") {
		if line == "x-backend-env: &backend-env" {
			inAnchor = true
			continue
		}
		if inAnchor && line != "" && !strings.HasPrefix(line, "  ") {
			break
		}
		if !inAnchor {
			continue
		}
		clean := strings.TrimSpace(line)
		key, _, ok := strings.Cut(clean, ":")
		if ok && isEnvKey(key) {
			keys[key] = true
		}
	}
	return sortedKeys(keys)
}

func composeServiceBlock(data, service string) string {
	lines := strings.Split(data, "\n")
	serviceLine := "  " + service + ":"
	for i, line := range lines {
		if line != serviceLine {
			continue
		}
		start := i
		end := len(lines)
		for j := i + 1; j < len(lines); j++ {
			if strings.HasPrefix(lines[j], "  ") && !strings.HasPrefix(lines[j], "    ") {
				end = j
				break
			}
		}
		return strings.Join(lines[start:end], "\n")
	}
	return ""
}

func missingKeys(want []string, got []string) []string {
	present := map[string]bool{}
	for _, key := range got {
		present[key] = true
	}

	var missing []string
	for _, key := range want {
		if !present[key] {
			missing = append(missing, key)
		}
	}
	return missing
}

func unexpectedKeys(want []string, got []string) []string {
	allowed := map[string]bool{}
	for _, key := range want {
		allowed[key] = true
	}

	var unexpected []string
	for _, key := range got {
		if !allowed[key] {
			unexpected = append(unexpected, key)
		}
	}
	return unexpected
}

func isEnvKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		if i > 0 && r == '_' {
			continue
		}
		return false
	}
	return true
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range configEnvKeys() {
		t.Setenv(key, "")
	}
}

func unsetConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range configEnvKeys() {
		t.Setenv(key, "")
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	}
}

func configEnvKeys() []string {
	return []string{
		"PORT",
		"DB_DRIVER",
		"DB_DSN",
		"DB_MAX_OPEN_CONNS",
		"DB_MAX_IDLE_CONNS",
		"DB_CONN_MAX_LIFETIME",
		"DB_CONN_MAX_IDLE_TIME",
		"REDIS_ADDR",
		"REDIS_PASSWORD",
		"REDIS_DB",
		"GIN_MODE",
		"CORS_ALLOW_ORIGINS",
		"TRUSTED_PROXIES",
		"HTTP_READ_TIMEOUT",
		"HTTP_READ_HEADER_TIMEOUT",
		"HTTP_WRITE_TIMEOUT",
		"HTTP_IDLE_TIMEOUT",
		"HTTP_SHUTDOWN_TIMEOUT",
		"HTTP_REQUEST_TIMEOUT",
		"HTTP_MAX_HEADER_BYTES",
		"HTTP_MAX_BODY_BYTES",
		"HTTP_METRICS_ENABLED",
		"HTTP_METRICS_TOKEN",
		"HTTP_HSTS_MAX_AGE_SECONDS",
		"HTTP_HSTS_INCLUDE_SUBDOMAINS",
		"HTTP_HSTS_PRELOAD",
		"HTTP_CROSS_ORIGIN_EMBEDDER_POLICY",
		"ADMIN_USERNAME",
		"ADMIN_PASSWORD",
		"ADMIN_NAME",
		"LOGIN_RATE_LIMIT_MAX_FAILURES",
		"LOGIN_RATE_LIMIT_WINDOW",
		"JWT_SECRET",
		"JWT_TTL",
		"JWT_CLOCK_SKEW",
		"JWT_ISSUER",
		"JWT_AUDIENCE",
	}
}

func restoreEnv(t *testing.T, key string) {
	t.Helper()
	original, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			if err := os.Setenv(key, original); err != nil {
				t.Fatalf("restore %s: %v", key, err)
			}
			return
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
	})
}
