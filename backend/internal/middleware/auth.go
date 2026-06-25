package middleware

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"github.com/QX-hao/HaoHaoAccounting/backend/internal/httputil"
)

const userContextKey = "user_id"

type jwtClaims struct {
	jwt.RegisteredClaims
}

type TokenRevocationChecker interface {
	IsTokenRevoked(ctx context.Context, token string) (bool, error)
}

type TokenService struct {
	secret   string
	ttl      time.Duration
	leeway   time.Duration
	issuer   string
	audience string
}

// NewTokenService 使用生产默认 issuer/audience 和有效期创建 JWT 服务。
func NewTokenService(secret string) (*TokenService, error) {
	return NewTokenServiceWithTTL(secret, 7*24*time.Hour, 30*time.Second, "haohao-accounting", "haohao-accounting-api")
}

// NewTokenServiceWithTTL 校验 JWT 密钥、有效期、时钟偏移、issuer 和 audience 后创建服务。
func NewTokenServiceWithTTL(secret string, ttl, leeway time.Duration, issuer, audience string) (*TokenService, error) {
	secret = strings.TrimSpace(secret)
	issuer = strings.TrimSpace(issuer)
	audience = strings.TrimSpace(audience)
	if secret == "" {
		return nil, errors.New("JWT_SECRET is required")
	}
	if len(secret) < 32 {
		return nil, errors.New("JWT_SECRET must be at least 32 characters")
	}
	if ttl <= 0 {
		return nil, errors.New("JWT_TTL must be positive")
	}
	if leeway < 0 {
		return nil, errors.New("JWT_CLOCK_SKEW must be non-negative")
	}
	if issuer == "" {
		return nil, errors.New("JWT_ISSUER is required")
	}
	if audience == "" {
		return nil, errors.New("JWT_AUDIENCE is required")
	}
	return &TokenService{secret: secret, ttl: ttl, leeway: leeway, issuer: issuer, audience: audience}, nil
}

// BuildToken 按默认有效期为用户生成 HS256 JWT。
func (s *TokenService) BuildToken(userID uint) (string, error) {
	return s.BuildTokenWithTTL(userID, s.ttl)
}

// BuildTokenWithTTL 按指定有效期生成带 subject、issuer、audience、iat 和 exp 的 HS256 JWT。
func (s *TokenService) BuildTokenWithTTL(userID uint, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		return "", errors.New("JWT_TTL must be positive")
	}
	now := time.Now()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(userID), 10),
			Issuer:    s.issuer,
			Audience:  jwt.ClaimStrings{s.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(s.secret))
}

// ParseToken 校验签名算法、过期时间、签发时间、issuer 和 audience，并返回正整数用户 id。
func (s *TokenService) ParseToken(token string) (uint, error) {
	claims, err := s.parseClaims(token)
	if err != nil {
		return 0, err
	}

	id, err := strconv.ParseUint(strings.TrimSpace(claims.Subject), 10, 64)
	if err != nil {
		return 0, err
	}
	if id == 0 || id > maxUint() {
		return 0, errors.New("token subject is out of range")
	}
	return uint(id), nil
}

// TokenExpiresAt 返回 token 的过期时间，供撤销缓存计算保留时长。
func (s *TokenService) TokenExpiresAt(token string) (time.Time, error) {
	claims, err := s.parseClaims(token)
	if err != nil {
		return time.Time{}, err
	}
	if claims.ExpiresAt == nil {
		return time.Time{}, errors.New("token expiration is required")
	}
	return claims.ExpiresAt.Time, nil
}

func maxUint() uint64 {
	if strconv.IntSize == 32 {
		return math.MaxUint32
	}
	return math.MaxUint64
}

// TokenRevocationExpiresAt 返回撤销记录应保留到的时间，包含解析 token 时允许的时钟偏移。
func (s *TokenService) TokenRevocationExpiresAt(token string) (time.Time, error) {
	expiresAt, err := s.TokenExpiresAt(token)
	if err != nil {
		return time.Time{}, err
	}
	return expiresAt.Add(s.leeway), nil
}

func (s *TokenService) parseClaims(token string) (*jwtClaims, error) {
	var claims jwtClaims
	parsed, err := jwt.ParseWithClaims(
		strings.TrimSpace(token),
		&claims,
		func(*jwt.Token) (any, error) {
			return []byte(s.secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(s.leeway),
		jwt.WithIssuer(s.issuer),
		jwt.WithAudience(s.audience),
	)
	if err != nil {
		return nil, err
	}
	if parsed == nil || !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return &claims, nil
}

func RequireAuth() gin.HandlerFunc {
	return RequireAuthWithRevocation(nil, nil)
}

// RequireAuthWithRevocation 校验 Bearer JWT，并在撤销检查失败或 token 已撤销时失败关闭。
func RequireAuthWithRevocation(checker TokenRevocationChecker, tokenService *TokenService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := BearerToken(c.GetHeader("Authorization"))
		if !ok {
			httputil.Unauthorized(c, "invalid Authorization format")
			c.Abort()
			return
		}
		if tokenService == nil {
			httputil.InvalidToken(c, "invalid token")
			c.Abort()
			return
		}
		userID, err := tokenService.ParseToken(token)
		if err != nil {
			httputil.InvalidToken(c, "invalid token")
			c.Abort()
			return
		}
		if checker != nil {
			revoked, err := checker.IsTokenRevoked(c.Request.Context(), token)
			if err != nil {
				_ = c.Error(err)
				httputil.InvalidToken(c, "unable to verify token")
				c.Abort()
				return
			}
			if revoked {
				httputil.InvalidToken(c, "invalid token")
				c.Abort()
				return
			}
		}
		c.Set(userContextKey, userID)
		c.Next()
	}
}

// BearerToken 按 RFC 6750 从 Authorization 头解析 Bearer token68 凭据。
func BearerToken(auth string) (string, bool) {
	scheme, token, ok := splitBearerCredentials(strings.TrimSpace(auth))
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	if !ValidBearerTokenValue(token) {
		return "", false
	}
	return token, true
}

func splitBearerCredentials(auth string) (string, string, bool) {
	separatorStart := strings.IndexByte(auth, ' ')
	if separatorStart <= 0 {
		return "", "", false
	}
	separatorEnd := separatorStart
	// 允许 scheme 和 token 之间有多个空格，但 token 内部不能再出现空格。
	for separatorEnd < len(auth) && auth[separatorEnd] == ' ' {
		separatorEnd++
	}
	if separatorEnd == len(auth) || strings.Contains(auth[separatorEnd:], " ") {
		return "", "", false
	}
	return auth[:separatorStart], auth[separatorEnd:], true
}

// ValidBearerTokenValue 校验 token68 字符集和尾部 padding 规则。
func ValidBearerTokenValue(token string) bool {
	if token == "" {
		return false
	}
	paddingStarted := false
	tokenStarted := false
	for _, r := range token {
		if r == '=' {
			// RFC 6750 的 token68 允许尾部 padding，但不能只有 padding，也不能在中间插入 padding。
			if !tokenStarted {
				return false
			}
			paddingStarted = true
			continue
		}
		if paddingStarted {
			return false
		}
		if !isBearerTokenChar(r) {
			return false
		}
		tokenStarted = true
	}
	return tokenStarted
}

func isBearerTokenChar(r rune) bool {
	return (r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= '0' && r <= '9') ||
		r == '-' ||
		r == '.' ||
		r == '_' ||
		r == '~' ||
		r == '+' ||
		r == '/'
}

// UserIDFromContext 读取认证中间件写入的用户 id，缺失或类型不匹配时返回 0。
func UserIDFromContext(c *gin.Context) uint {
	v, ok := c.Get(userContextKey)
	if !ok {
		return 0
	}
	id, ok := v.(uint)
	if !ok {
		return 0
	}
	return id
}
