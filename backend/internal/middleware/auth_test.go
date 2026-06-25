package middleware

import (
	"context"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestNewTokenServiceRequiresStrongSecret(t *testing.T) {
	if _, err := NewTokenService(""); err == nil {
		t.Fatal("expected missing secret error")
	}

	if _, err := NewTokenService("short"); err == nil {
		t.Fatal("expected short secret error")
	}

	if _, err := NewTokenServiceWithTTL("test-jwt-secret-with-at-least-32-chars", 0, time.Second, "issuer", "audience"); err == nil {
		t.Fatal("expected ttl error")
	}
	if _, err := NewTokenServiceWithTTL("test-jwt-secret-with-at-least-32-chars", time.Hour, -time.Second, "issuer", "audience"); err == nil {
		t.Fatal("expected leeway error")
	}
	if _, err := NewTokenServiceWithTTL("test-jwt-secret-with-at-least-32-chars", time.Hour, time.Second, "", "audience"); err == nil {
		t.Fatal("expected issuer error")
	}
	if _, err := NewTokenServiceWithTTL("test-jwt-secret-with-at-least-32-chars", time.Hour, time.Second, "issuer", ""); err == nil {
		t.Fatal("expected audience error")
	}
}

func TestBuildAndParseToken(t *testing.T) {
	tokenService := testTokenService(t)

	token, err := tokenService.BuildToken(42)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	userID, err := tokenService.ParseToken(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if userID != 42 {
		t.Fatalf("userID = %d", userID)
	}
}

func TestTokenExpiresAtReturnsTokenExpiration(t *testing.T) {
	tokenService := testTokenService(t)

	before := time.Now().Add(29 * time.Minute)
	token, err := tokenService.BuildTokenWithTTL(42, 30*time.Minute)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	expiresAt, err := tokenService.TokenExpiresAt(token)
	if err != nil {
		t.Fatalf("token expires at: %v", err)
	}
	after := time.Now().Add(31 * time.Minute)
	if expiresAt.Before(before) || expiresAt.After(after) {
		t.Fatalf("expiresAt = %s, want between %s and %s", expiresAt, before, after)
	}
}

func TestTokenRevocationExpiresAtIncludesLeeway(t *testing.T) {
	tokenService, err := NewTokenServiceWithTTL("test-jwt-secret-with-at-least-32-chars", time.Hour, 30*time.Second, "haohao-accounting", "haohao-accounting-api")
	if err != nil {
		t.Fatalf("token service: %v", err)
	}

	token, err := tokenService.BuildTokenWithTTL(42, time.Hour)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	expiresAt, err := tokenService.TokenExpiresAt(token)
	if err != nil {
		t.Fatalf("token expires at: %v", err)
	}
	revocationExpiresAt, err := tokenService.TokenRevocationExpiresAt(token)
	if err != nil {
		t.Fatalf("token revocation expires at: %v", err)
	}
	if got := revocationExpiresAt.Sub(expiresAt); got != 30*time.Second {
		t.Fatalf("revocation leeway = %s", got)
	}
}

func TestParseTokenRejectsExpiredToken(t *testing.T) {
	tokenService := testTokenService(t)

	now := time.Now()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "42",
			Issuer:    tokenService.issuer,
			Audience:  jwt.ClaimStrings{tokenService.audience},
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(tokenService.secret))
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	if _, err := tokenService.ParseToken(token); err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestParseTokenAcceptsSmallClockSkew(t *testing.T) {
	tokenService, err := NewTokenServiceWithTTL("test-jwt-secret-with-at-least-32-chars", time.Hour, time.Minute, "haohao-accounting", "haohao-accounting-api")
	if err != nil {
		t.Fatalf("token service: %v", err)
	}

	now := time.Now()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "42",
			Issuer:    tokenService.issuer,
			Audience:  jwt.ClaimStrings{tokenService.audience},
			IssuedAt:  jwt.NewNumericDate(now.Add(-time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-30 * time.Second)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(tokenService.secret))
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	if _, err := tokenService.ParseToken(token); err != nil {
		t.Fatalf("parse token with leeway: %v", err)
	}
}

func TestParseTokenRejectsUnexpectedSigningMethod(t *testing.T) {
	tokenService := testTokenService(t)
	now := time.Now()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "42",
			Issuer:    tokenService.issuer,
			Audience:  jwt.ClaimStrings{tokenService.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS384, claims).SignedString([]byte(tokenService.secret))
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	if _, err := tokenService.ParseToken(token); err == nil {
		t.Fatal("expected signing method error")
	}
}

func TestParseTokenRejectsUnexpectedIssuerAndAudience(t *testing.T) {
	tokenService := testTokenService(t)
	now := time.Now()
	tests := []struct {
		name     string
		issuer   string
		audience jwt.ClaimStrings
	}{
		{name: "issuer", issuer: "other-issuer", audience: jwt.ClaimStrings{tokenService.audience}},
		{name: "audience", issuer: tokenService.issuer, audience: jwt.ClaimStrings{"other-audience"}},
		{name: "missing audience", issuer: tokenService.issuer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := jwtClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject:   "42",
					Issuer:    tt.issuer,
					Audience:  tt.audience,
					IssuedAt:  jwt.NewNumericDate(now),
					ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
				},
			}
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(tokenService.secret))
			if err != nil {
				t.Fatalf("build token: %v", err)
			}
			if _, err := tokenService.ParseToken(token); err == nil {
				t.Fatal("expected issuer/audience error")
			}
		})
	}
}

func TestParseTokenRejectsSubjectOutsideUintRange(t *testing.T) {
	tokenService := testTokenService(t)
	now := time.Now()
	subject := strconv.FormatUint(uint64(math.MaxUint32)+1, 10)
	if strconv.IntSize == 64 {
		subject = "18446744073709551616"
	}
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			Issuer:    tokenService.issuer,
			Audience:  jwt.ClaimStrings{tokenService.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(tokenService.secret))
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	if _, err := tokenService.ParseToken(token); err == nil {
		t.Fatal("expected subject range error")
	}
}

func TestParseTokenRejectsZeroSubject(t *testing.T) {
	tokenService := testTokenService(t)
	now := time.Now()
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "0",
			Issuer:    tokenService.issuer,
			Audience:  jwt.ClaimStrings{tokenService.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(tokenService.secret))
	if err != nil {
		t.Fatalf("build token: %v", err)
	}
	if _, err := tokenService.ParseToken(token); err == nil {
		t.Fatal("expected zero subject error")
	}
}

func TestRequireAuthRejectsRevokedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokenService := testTokenService(t)

	token, err := tokenService.BuildToken(42)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}

	router := gin.New()
	router.GET("/private", RequireAuthWithRevocation(&staticRevocationChecker{revoked: true}, tokenService), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
}

func TestRequireAuthSkipsRevocationCheckForInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	checker := &staticRevocationChecker{}
	router := gin.New()
	router.GET("/private", RequireAuthWithRevocation(checker, testTokenService(t)), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
	if got := resp.Header().Get("WWW-Authenticate"); got != `Bearer realm="haohao-accounting-api", error="invalid_token", error_description="The access token is missing, expired, revoked, or invalid"` {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
	if checker.calls != 0 {
		t.Fatalf("revocation checker calls = %d, want 0", checker.calls)
	}
}

func TestRequireAuthFailsClosedWhenRevocationCheckErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokenService := testTokenService(t)

	token, err := tokenService.BuildToken(42)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}

	called := false
	router := gin.New()
	router.GET("/private", RequireAuthWithRevocation(&staticRevocationChecker{err: errors.New("redis unavailable")}, tokenService), func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if called {
		t.Fatal("handler was called after revocation check error")
	}
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
	if got := resp.Header().Get("WWW-Authenticate"); got != `Bearer realm="haohao-accounting-api", error="invalid_token", error_description="The access token is missing, expired, revoked, or invalid"` {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestRequireAuthStoresTokenAndRemovesAuthorizationHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokenService := testTokenService(t)

	token, err := tokenService.BuildToken(42)
	if err != nil {
		t.Fatalf("build token: %v", err)
	}

	called := false
	router := gin.New()
	router.GET("/private", RequireAuthWithRevocation(nil, tokenService), func(c *gin.Context) {
		called = true
		if got := c.GetHeader("Authorization"); got != "" {
			t.Fatalf("Authorization header leaked to handler: %q", got)
		}
		contextToken, ok := BearerTokenFromContext(c)
		if !ok || contextToken != token {
			t.Fatalf("BearerTokenFromContext = %q, %v, want token", contextToken, ok)
		}
		if got := UserIDFromContext(c); got != 42 {
			t.Fatalf("user id = %d, want 42", got)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if !called {
		t.Fatal("handler was not called")
	}
	if resp.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.Code)
	}
}

func TestBearerTokenFromContextRejectsInvalidContextValues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, tc := range []struct {
		name      string
		value     any
		setValue  bool
		wantToken string
		wantOK    bool
	}{
		{name: "missing"},
		{name: "wrong type", setValue: true, value: 42},
		{name: "empty", setValue: true, value: ""},
		{name: "invalid character", setValue: true, value: "token,part"},
		{name: "padding only", setValue: true, value: "=="},
		{name: "too long", setValue: true, value: strings.Repeat("a", maxBearerTokenLength+1)},
		{name: "valid", setValue: true, value: "header.payload.signature", wantToken: "header.payload.signature", wantOK: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			if tc.setValue {
				c.Set(bearerTokenContextKey, tc.value)
			}

			token, ok := BearerTokenFromContext(c)
			if token != tc.wantToken || ok != tc.wantOK {
				t.Fatalf("BearerTokenFromContext = %q, %v, want %q, %v", token, ok, tc.wantToken, tc.wantOK)
			}
		})
	}
}

func TestRequireAuthSetsAuthenticateChallenge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/private", RequireAuthWithRevocation(nil, testTokenService(t)), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/private", nil))

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
	if got := resp.Header().Get("WWW-Authenticate"); got != `Bearer realm="haohao-accounting-api"` {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestRequireAuthRejectsMalformedBearerHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/private", RequireAuthWithRevocation(nil, testTokenService(t)), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer token extra")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.Code)
	}
	if got := resp.Header().Get("WWW-Authenticate"); got != `Bearer realm="haohao-accounting-api"` {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestBearerTokenParsesRFC6750Credentials(t *testing.T) {
	for _, tc := range []struct {
		name      string
		header    string
		wantToken string
		wantOK    bool
	}{
		{name: "jwt", header: "Bearer header.payload.signature", wantToken: "header.payload.signature", wantOK: true},
		{name: "case insensitive scheme", header: "bearer abc-._~+/==", wantToken: "abc-._~+/==", wantOK: true},
		{name: "multiple spaces", header: "Bearer   token", wantToken: "token", wantOK: true},
		{name: "missing token", header: "Bearer", wantOK: false},
		{name: "wrong scheme", header: "Basic token", wantOK: false},
		{name: "extra field", header: "Bearer token extra", wantOK: false},
		{name: "tab separator", header: "Bearer\ttoken", wantOK: false},
		{name: "newline separator", header: "Bearer\ntoken", wantOK: false},
		{name: "invalid character", header: "Bearer token,part", wantOK: false},
		{name: "padding only", header: "Bearer ==", wantOK: false},
		{name: "padding before token char", header: "Bearer abc=def", wantOK: false},
		{name: "max length token", header: "Bearer " + strings.Repeat("a", maxBearerTokenLength), wantToken: strings.Repeat("a", maxBearerTokenLength), wantOK: true},
		{name: "too long token", header: "Bearer " + strings.Repeat("a", maxBearerTokenLength+1), wantOK: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token, ok := BearerToken(tc.header)
			if ok != tc.wantOK || token != tc.wantToken {
				t.Fatalf("BearerToken(%q) = %q, %v, want %q, %v", tc.header, token, ok, tc.wantToken, tc.wantOK)
			}
		})
	}
}

type staticRevocationChecker struct {
	revoked bool
	err     error
	calls   int
}

func (c *staticRevocationChecker) IsTokenRevoked(context.Context, string) (bool, error) {
	c.calls++
	return c.revoked, c.err
}

func testTokenService(t *testing.T) *TokenService {
	t.Helper()
	tokenService, err := NewTokenService("test-jwt-secret-with-at-least-32-chars")
	if err != nil {
		t.Fatalf("token service: %v", err)
	}
	return tokenService
}
