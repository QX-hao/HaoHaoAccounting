package config

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
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
		cfg.HTTP.RequestTimeout != 0 ||
		cfg.HTTP.MaxHeaderBytes != 1<<20 ||
		cfg.HTTP.MaxBodyBytes != 6*1024*1024 ||
		cfg.HTTP.HSTSMaxAgeSeconds != 0 ||
		cfg.HTTP.HSTSIncludeSubDomains ||
		cfg.HTTP.HSTSPreload {
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
	t.Setenv("HTTP_HSTS_MAX_AGE_SECONDS", "31536000")
	t.Setenv("HTTP_HSTS_INCLUDE_SUBDOMAINS", "true")
	t.Setenv("HTTP_HSTS_PRELOAD", "true")
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
		cfg.HTTP.HSTSMaxAgeSeconds != 31536000 ||
		!cfg.HTTP.HSTSIncludeSubDomains ||
		!cfg.HTTP.HSTSPreload {
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
	t.Setenv("HTTP_HSTS_INCLUDE_SUBDOMAINS", "not-a-bool")
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
		"HTTP_HSTS_INCLUDE_SUBDOMAINS",
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
	invalidHSTSDirective := valid
	invalidHSTSDirective.HSTSIncludeSubDomains = true
	if err := invalidHSTSDirective.Validate(); err == nil {
		t.Fatal("expected hsts directive without max-age error")
	}
	invalidHSTSPreload := valid
	invalidHSTSPreload.HSTSPreload = true
	if err := invalidHSTSPreload.Validate(); err == nil {
		t.Fatal("expected hsts preload without max-age error")
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
	if err := (LoginRateLimitConfig{MaxFailures: 0, Window: time.Minute}).Validate(); err == nil {
		t.Fatal("expected invalid max failures error")
	}
	if err := (LoginRateLimitConfig{MaxFailures: 1, Window: 0}).Validate(); err == nil {
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
	multipleInvalid.LoginRateLimit.Window = 0
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

func TestConfigEnvironmentKeysStayDocumented(t *testing.T) {
	sourceKeys := configSourceEnvKeys(t)
	exampleKeys := envExampleKeys(t)

	if missing := missingKeys(sourceKeys, exampleKeys); len(missing) > 0 {
		t.Fatalf("backend/.env.example is missing config keys: %v", missing)
	}
}

func TestComposeBackendEnvironmentCoversConfigKeys(t *testing.T) {
	sourceKeys := configSourceEnvKeys(t)
	composeKeys := composeBackendEnvironmentKeys(t)

	if missing := missingKeys(sourceKeys, composeKeys); len(missing) > 0 {
		t.Fatalf("docker-compose.yaml backend environment is missing config keys: %v", missing)
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
	if cfg.HTTP.RequestTimeout != 0 {
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
		RequestTimeout:    0,
		MaxHeaderBytes:    1 << 20,
		MaxBodyBytes:      6 * 1024 * 1024,
	}
}

func configSourceEnvKeys(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatalf("read config.go: %v", err)
	}

	keyPattern := regexp.MustCompile(`"(?:[A-Z][A-Z0-9]*_)*[A-Z][A-Z0-9]*"`)
	ignored := map[string]bool{
		"POSTGRES": true,
	}
	keys := map[string]bool{}
	for _, match := range keyPattern.FindAllString(string(data), -1) {
		key := strings.Trim(match, `"`)
		if ignored[key] {
			continue
		}
		if strings.Contains(key, "_") || strings.HasPrefix(key, "PORT") {
			keys[key] = true
		}
	}
	return sortedKeys(keys)
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
		if key != "" {
			keys[key] = true
		}
	}
	return sortedKeys(keys)
}

func composeBackendEnvironmentKeys(t *testing.T) []string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "docker-compose.yaml"))
	if err != nil {
		t.Fatalf("read docker-compose.yaml: %v", err)
	}

	keys := map[string]bool{}
	inBackend, inEnvironment := false, false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "  backend:") {
			inBackend = true
			inEnvironment = false
			continue
		}
		if inBackend && strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "  backend:") {
			break
		}
		if !inBackend {
			continue
		}
		if strings.HasPrefix(line, "    environment:") {
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
		if ok && key != "" {
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
	for _, key := range []string{
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
		"HTTP_HSTS_MAX_AGE_SECONDS",
		"HTTP_HSTS_INCLUDE_SUBDOMAINS",
		"HTTP_HSTS_PRELOAD",
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
	} {
		t.Setenv(key, "")
	}
}
