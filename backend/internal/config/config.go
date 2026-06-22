package config

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPostgresDSN        = "host=127.0.0.1 user=postgres password=haohao123 dbname=haohaoaccounting port=55432 sslmode=disable TimeZone=Asia/Shanghai"
	defaultMySQLDSN           = "root:root@tcp(127.0.0.1:53306)/haohaoaccounting?charset=utf8mb4&parseTime=True&loc=Local"
	defaultDBMaxOpenConns     = 25
	defaultDBMaxIdleConns     = 10
	defaultDBConnMaxLifetime  = time.Hour
	defaultDBConnMaxIdleTime  = 30 * time.Minute
	defaultHTTPRequestTimeout = 60 * time.Second
	minJWTSecretLength        = 32
	hstsPreloadMinMaxAge      = 31536000
)

type Config struct {
	Port           string
	Database       DatabaseConfig
	Redis          RedisConfig
	HTTP           HTTPConfig
	Admin          AdminConfig
	LoginRateLimit LoginRateLimitConfig
	JWT            JWTConfig
}

type DatabaseConfig struct {
	Driver          string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type HTTPConfig struct {
	GinMode               string
	CORSAllowOrigins      []string
	TrustedProxies        []string
	ReadTimeout           time.Duration
	ReadHeaderTimeout     time.Duration
	WriteTimeout          time.Duration
	IdleTimeout           time.Duration
	ShutdownTimeout       time.Duration
	RequestTimeout        time.Duration
	MaxHeaderBytes        int
	MaxBodyBytes          int64
	HSTSMaxAgeSeconds     int
	HSTSIncludeSubDomains bool
	HSTSPreload           bool
}

type AdminConfig struct {
	Username string
	Password string
	Name     string
}

type LoginRateLimitConfig struct {
	MaxFailures int
	Window      time.Duration
}

type JWTConfig struct {
	Secret    string
	TTL       time.Duration
	ClockSkew time.Duration
	Issuer    string
	Audience  string
}

func LoadDotEnv(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, line := range strings.Split(string(data), "\n") {
		clean := strings.TrimSpace(line)
		clean = strings.TrimPrefix(clean, "\ufeff")
		if clean == "" || strings.HasPrefix(clean, "#") {
			continue
		}
		clean = trimDotEnvExportPrefix(clean)
		key, value, ok := strings.Cut(clean, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = parseDotEnvValue(value)
		if _, exists := os.LookupEnv(key); key == "" || exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return nil
}

func trimDotEnvExportPrefix(line string) string {
	if len(line) <= len("export") || !strings.HasPrefix(line, "export") {
		return line
	}
	if !isDotEnvSpace(rune(line[len("export")])) {
		return line
	}
	return strings.TrimSpace(line[len("export"):])
}

func parseDotEnvValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	quote := value[0]
	if quote == '"' || quote == '\'' {
		return parseQuotedDotEnvValue(value, quote)
	}
	return strings.TrimSpace(stripDotEnvInlineComment(value))
}

func parseQuotedDotEnvValue(value string, quote byte) string {
	var builder strings.Builder
	escaped := false
	for i := 1; i < len(value); i++ {
		char := value[i]
		if quote == '"' && escaped {
			switch char {
			case 'n':
				builder.WriteByte('\n')
			case 'r':
				builder.WriteByte('\r')
			case '"', '\\':
				builder.WriteByte(char)
			default:
				builder.WriteByte('\\')
				builder.WriteByte(char)
			}
			escaped = false
			continue
		}
		if quote == '"' && char == '\\' {
			escaped = true
			continue
		}
		if char == quote {
			return builder.String()
		}
		builder.WriteByte(char)
	}
	if quote == '"' && escaped {
		builder.WriteByte('\\')
	}
	return strings.Trim(value, `"'`)
}

func stripDotEnvInlineComment(value string) string {
	for i, char := range value {
		if char == '#' && i > 0 && isDotEnvSpace(rune(value[i-1])) {
			return value[:i]
		}
	}
	return value
}

func isDotEnvSpace(char rune) bool {
	return char == ' ' || char == '\t'
}

func Load() Config {
	driver := stringEnv("DB_DRIVER", "postgres")
	return Config{
		Port: stringEnv("PORT", "8080"),
		Database: DatabaseConfig{
			Driver:          driver,
			DSN:             databaseDSN(driver),
			MaxOpenConns:    positiveIntEnv("DB_MAX_OPEN_CONNS", defaultDBMaxOpenConns),
			MaxIdleConns:    positiveIntEnv("DB_MAX_IDLE_CONNS", defaultDBMaxIdleConns),
			ConnMaxLifetime: durationEnv("DB_CONN_MAX_LIFETIME", defaultDBConnMaxLifetime),
			ConnMaxIdleTime: durationEnv("DB_CONN_MAX_IDLE_TIME", defaultDBConnMaxIdleTime),
		},
		Redis: RedisConfig{
			Addr:     stringEnv("REDIS_ADDR", "127.0.0.1:56379"),
			Password: stringEnv("REDIS_PASSWORD", "haohao123"),
			DB:       intEnv("REDIS_DB", 0),
		},
		HTTP: HTTPConfig{
			GinMode:               stringEnv("GIN_MODE", "release"),
			CORSAllowOrigins:      corsAllowOrigins(),
			TrustedProxies:        csvEnv("TRUSTED_PROXIES"),
			ReadTimeout:           durationEnv("HTTP_READ_TIMEOUT", 15*time.Second),
			ReadHeaderTimeout:     durationEnv("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			WriteTimeout:          durationEnv("HTTP_WRITE_TIMEOUT", 30*time.Second),
			IdleTimeout:           durationEnv("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:       durationEnv("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
			RequestTimeout:        nonNegativeDurationEnv("HTTP_REQUEST_TIMEOUT", defaultHTTPRequestTimeout),
			MaxHeaderBytes:        positiveIntEnv("HTTP_MAX_HEADER_BYTES", 1<<20),
			MaxBodyBytes:          int64Env("HTTP_MAX_BODY_BYTES", 6*1024*1024),
			HSTSMaxAgeSeconds:     nonNegativeIntEnv("HTTP_HSTS_MAX_AGE_SECONDS", 0),
			HSTSIncludeSubDomains: boolEnv("HTTP_HSTS_INCLUDE_SUBDOMAINS", false),
			HSTSPreload:           boolEnv("HTTP_HSTS_PRELOAD", false),
		},
		Admin: AdminConfig{
			Username: strings.TrimSpace(os.Getenv("ADMIN_USERNAME")),
			Password: strings.TrimSpace(os.Getenv("ADMIN_PASSWORD")),
			Name:     stringEnv("ADMIN_NAME", "好好用户"),
		},
		LoginRateLimit: LoginRateLimitConfig{
			MaxFailures: intEnv("LOGIN_RATE_LIMIT_MAX_FAILURES", 5),
			Window:      durationEnv("LOGIN_RATE_LIMIT_WINDOW", 10*time.Minute),
		},
		JWT: JWTConfig{
			Secret:    strings.TrimSpace(os.Getenv("JWT_SECRET")),
			TTL:       durationEnv("JWT_TTL", 7*24*time.Hour),
			ClockSkew: nonNegativeDurationEnv("JWT_CLOCK_SKEW", 30*time.Second),
			Issuer:    stringEnv("JWT_ISSUER", "haohao-accounting"),
			Audience:  stringEnv("JWT_AUDIENCE", "haohao-accounting-api"),
		},
	}
}

func LoadStrict() (Config, error) {
	cfg := Load()
	if err := validateEnvValues(); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var errs []error
	if err := validatePort(c.Port); err != nil {
		errs = append(errs, err)
	}
	if err := c.Database.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.Redis.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.HTTP.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.LoginRateLimit.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.JWT.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.Admin.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func validatePort(port string) error {
	port = strings.TrimSpace(port)
	if port == "" {
		return errors.New("PORT is required")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("PORT must be an integer between 1 and 65535")
	}
	return nil
}

func validateEnvValues() error {
	var errs []error
	checks := []func() error{
		validatePortEnv("PORT"),
		validatePositiveIntEnv("DB_MAX_OPEN_CONNS"),
		validatePositiveIntEnv("DB_MAX_IDLE_CONNS"),
		validatePositiveDurationEnv("DB_CONN_MAX_LIFETIME"),
		validatePositiveDurationEnv("DB_CONN_MAX_IDLE_TIME"),
		validateIntEnv("REDIS_DB"),
		validatePositiveDurationEnv("HTTP_READ_TIMEOUT"),
		validatePositiveDurationEnv("HTTP_READ_HEADER_TIMEOUT"),
		validatePositiveDurationEnv("HTTP_WRITE_TIMEOUT"),
		validatePositiveDurationEnv("HTTP_IDLE_TIMEOUT"),
		validatePositiveDurationEnv("HTTP_SHUTDOWN_TIMEOUT"),
		validateNonNegativeDurationEnv("HTTP_REQUEST_TIMEOUT"),
		validatePositiveIntEnv("HTTP_MAX_HEADER_BYTES"),
		validatePositiveInt64Env("HTTP_MAX_BODY_BYTES"),
		validateNonNegativeIntEnv("HTTP_HSTS_MAX_AGE_SECONDS"),
		validateBoolEnv("HTTP_HSTS_INCLUDE_SUBDOMAINS"),
		validateBoolEnv("HTTP_HSTS_PRELOAD"),
		validatePositiveIntEnv("LOGIN_RATE_LIMIT_MAX_FAILURES"),
		validatePositiveDurationEnv("LOGIN_RATE_LIMIT_WINDOW"),
		validatePositiveDurationEnv("JWT_TTL"),
		validateNonNegativeDurationEnv("JWT_CLOCK_SKEW"),
	}
	for _, check := range checks {
		if err := check(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func validatePortEnv(key string) func() error {
	return func() error {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			return nil
		}
		if err := validatePort(value); err != nil {
			return err
		}
		return nil
	}
}

func validateIntEnv(key string) func() error {
	return func() error {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			return nil
		}
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("%s must be an integer", key)
		}
		return nil
	}
}

func validateNonNegativeIntEnv(key string) func() error {
	return func() error {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			return nil
		}
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("%s must be a non-negative integer", key)
		}
		return nil
	}
}

func validatePositiveIntEnv(key string) func() error {
	return func() error {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			return nil
		}
		n, err := strconv.Atoi(value)
		if err != nil || n <= 0 {
			return fmt.Errorf("%s must be a positive integer", key)
		}
		return nil
	}
}

func validatePositiveInt64Env(key string) func() error {
	return func() error {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			return nil
		}
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || n <= 0 {
			return fmt.Errorf("%s must be a positive integer", key)
		}
		return nil
	}
}

func validateBoolEnv(key string) func() error {
	return func() error {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			return nil
		}
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("%s must be a boolean", key)
		}
		return nil
	}
}

func validatePositiveDurationEnv(key string) func() error {
	return func() error {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			return nil
		}
		duration, err := time.ParseDuration(value)
		if err != nil || duration <= 0 {
			return fmt.Errorf("%s must be a positive Go duration", key)
		}
		return nil
	}
}

func validateNonNegativeDurationEnv(key string) func() error {
	return func() error {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "" {
			return nil
		}
		duration, err := time.ParseDuration(value)
		if err != nil || duration < 0 {
			return fmt.Errorf("%s must be a non-negative Go duration", key)
		}
		return nil
	}
}

func (c DatabaseConfig) Validate() error {
	var errs []error
	switch strings.ToLower(strings.TrimSpace(c.Driver)) {
	case "postgres", "pgsql", "mysql":
	default:
		errs = append(errs, errors.New("DB_DRIVER must be one of: postgres, pgsql, mysql"))
	}
	if c.MaxOpenConns <= 0 {
		errs = append(errs, errors.New("DB_MAX_OPEN_CONNS must be positive"))
	}
	if c.MaxIdleConns <= 0 {
		errs = append(errs, errors.New("DB_MAX_IDLE_CONNS must be positive"))
	}
	if c.MaxOpenConns > 0 && c.MaxIdleConns > c.MaxOpenConns {
		errs = append(errs, errors.New("DB_MAX_IDLE_CONNS must be less than or equal to DB_MAX_OPEN_CONNS"))
	}
	if c.ConnMaxLifetime <= 0 {
		errs = append(errs, errors.New("DB_CONN_MAX_LIFETIME must be positive"))
	}
	if c.ConnMaxIdleTime <= 0 {
		errs = append(errs, errors.New("DB_CONN_MAX_IDLE_TIME must be positive"))
	}
	return errors.Join(errs...)
}

func (c RedisConfig) Validate() error {
	var errs []error
	host, err := redisHost(c.Addr)
	if err != nil {
		errs = append(errs, err)
	}
	if c.DB < 0 {
		errs = append(errs, errors.New("REDIS_DB must be non-negative"))
	}
	if err == nil && !isLoopbackRedisHost(host) && strings.TrimSpace(c.Password) == "" {
		errs = append(errs, errors.New("REDIS_PASSWORD is required when REDIS_ADDR is not loopback"))
	}
	return errors.Join(errs...)
}

func redisHost(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", errors.New("REDIS_ADDR is required")
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return "", errors.New("REDIS_ADDR must be in host:port format")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return "", errors.New("REDIS_ADDR host is required")
	}
	return host, nil
}

func isLoopbackRedisHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}

func (c HTTPConfig) Validate() error {
	var errs []error
	switch strings.ToLower(strings.TrimSpace(c.GinMode)) {
	case "debug", "release", "test":
	default:
		errs = append(errs, errors.New("GIN_MODE must be one of: debug, release, test"))
	}
	if c.ReadTimeout <= 0 {
		errs = append(errs, errors.New("HTTP_READ_TIMEOUT must be positive"))
	}
	if c.ReadHeaderTimeout <= 0 {
		errs = append(errs, errors.New("HTTP_READ_HEADER_TIMEOUT must be positive"))
	}
	if c.WriteTimeout <= 0 {
		errs = append(errs, errors.New("HTTP_WRITE_TIMEOUT must be positive"))
	}
	if c.IdleTimeout <= 0 {
		errs = append(errs, errors.New("HTTP_IDLE_TIMEOUT must be positive"))
	}
	if c.ShutdownTimeout <= 0 {
		errs = append(errs, errors.New("HTTP_SHUTDOWN_TIMEOUT must be positive"))
	}
	if c.MaxHeaderBytes <= 0 {
		errs = append(errs, errors.New("HTTP_MAX_HEADER_BYTES must be positive"))
	}
	if c.MaxBodyBytes <= 0 {
		errs = append(errs, errors.New("HTTP_MAX_BODY_BYTES must be positive"))
	}
	if c.HSTSMaxAgeSeconds < 0 {
		errs = append(errs, errors.New("HTTP_HSTS_MAX_AGE_SECONDS must be non-negative"))
	}
	if c.RequestTimeout < 0 {
		errs = append(errs, errors.New("HTTP_REQUEST_TIMEOUT must be non-negative"))
	}
	if err := validateTrustedProxies(c.TrustedProxies); err != nil {
		errs = append(errs, err)
	}
	if c.HSTSMaxAgeSeconds == 0 && c.HSTSIncludeSubDomains {
		errs = append(errs, errors.New("HTTP_HSTS_MAX_AGE_SECONDS must be positive when HSTS includeSubDomains is enabled"))
	}
	if c.HSTSPreload {
		if c.HSTSMaxAgeSeconds < hstsPreloadMinMaxAge {
			errs = append(errs, fmt.Errorf("HTTP_HSTS_MAX_AGE_SECONDS must be at least %d when HTTP_HSTS_PRELOAD is enabled", hstsPreloadMinMaxAge))
		}
		if !c.HSTSIncludeSubDomains {
			errs = append(errs, errors.New("HTTP_HSTS_INCLUDE_SUBDOMAINS must be true when HTTP_HSTS_PRELOAD is enabled"))
		}
	}
	return errors.Join(errs...)
}

func validateTrustedProxies(proxies []string) error {
	for _, proxy := range proxies {
		proxy = strings.TrimSpace(proxy)
		if proxy == "" {
			continue
		}
		if strings.Contains(proxy, "/") {
			prefix, err := netip.ParsePrefix(proxy)
			if err != nil {
				return fmt.Errorf("TRUSTED_PROXIES entry %q must be an IP address or CIDR", proxy)
			}
			if prefix.Addr().IsUnspecified() {
				return fmt.Errorf("TRUSTED_PROXIES entry %q must not use an unspecified address", proxy)
			}
			if prefix.Bits() == 0 {
				return fmt.Errorf("TRUSTED_PROXIES entry %q must not trust all addresses", proxy)
			}
			continue
		}
		addr, err := netip.ParseAddr(proxy)
		if err != nil {
			return fmt.Errorf("TRUSTED_PROXIES entry %q must be an IP address or CIDR", proxy)
		}
		if addr.IsUnspecified() {
			return fmt.Errorf("TRUSTED_PROXIES entry %q must not use an unspecified address", proxy)
		}
	}
	return nil
}

func (c AdminConfig) Validate() error {
	if strings.TrimSpace(c.Username) == "" {
		return errors.New("ADMIN_USERNAME is required")
	}
	if strings.TrimSpace(c.Password) == "" {
		return errors.New("ADMIN_PASSWORD is required")
	}
	return nil
}

func (c LoginRateLimitConfig) Validate() error {
	var errs []error
	if c.MaxFailures <= 0 {
		errs = append(errs, errors.New("LOGIN_RATE_LIMIT_MAX_FAILURES must be positive"))
	}
	if c.Window <= 0 {
		errs = append(errs, errors.New("LOGIN_RATE_LIMIT_WINDOW must be positive"))
	}
	return errors.Join(errs...)
}

func (c JWTConfig) Validate() error {
	if c.Secret == "" {
		return errors.New("JWT_SECRET is required")
	}
	if len(c.Secret) < minJWTSecretLength {
		return errors.New("JWT_SECRET must be at least 32 characters")
	}
	if c.TTL <= 0 {
		return errors.New("JWT_TTL must be positive")
	}
	if c.ClockSkew < 0 {
		return errors.New("JWT_CLOCK_SKEW must be non-negative")
	}
	if strings.TrimSpace(c.Issuer) == "" {
		return errors.New("JWT_ISSUER is required")
	}
	if strings.TrimSpace(c.Audience) == "" {
		return errors.New("JWT_AUDIENCE is required")
	}
	return nil
}

func ExplicitDatabaseDSN() string {
	return strings.TrimSpace(os.Getenv("DB_DSN"))
}

func databaseDSN(driver string) string {
	if dsn := strings.TrimSpace(os.Getenv("DB_DSN")); dsn != "" {
		return dsn
	}
	if strings.EqualFold(driver, "mysql") {
		return defaultMySQLDSN
	}
	return defaultPostgresDSN
}

func corsAllowOrigins() []string {
	origins := csvEnv("CORS_ALLOW_ORIGINS")
	if len(origins) > 0 {
		return origins
	}
	return []string{"http://localhost:3000", "http://127.0.0.1:3000"}
}

func stringEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func nonNegativeIntEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func positiveIntEnv(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func boolEnv(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func int64Env(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}

func nonNegativeDurationEnv(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < 0 {
		return fallback
	}
	return duration
}

func csvEnv(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if clean := strings.TrimSpace(part); clean != "" {
			values = append(values, clean)
		}
	}
	return values
}
