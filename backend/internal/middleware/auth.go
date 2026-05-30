package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const userContextKey = "user_id"
const defaultJWTSecret = "haohao-dev-jwt-secret"

type jwtClaims struct {
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func BuildToken(userID uint) string {
	now := time.Now()
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	claims := jwtClaims{
		Subject:   strconv.FormatUint(uint64(userID), 10),
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(7 * 24 * time.Hour).Unix(),
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)
	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	return unsigned + "." + signJWT(unsigned)
}

func ParseToken(token string) (uint, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, errors.New("invalid token")
	}

	unsigned := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(signJWT(unsigned))) {
		return 0, errors.New("invalid token signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, err
	}

	var claims jwtClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, err
	}
	if claims.ExpiresAt <= time.Now().Unix() {
		return 0, errors.New("token expired")
	}

	id, err := strconv.ParseUint(strings.TrimSpace(claims.Subject), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

func signJWT(unsigned string) string {
	mac := hmac.New(sha256.New, []byte(jwtSecret()))
	mac.Write([]byte(unsigned))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func jwtSecret() string {
	if value := strings.TrimSpace(os.Getenv("JWT_SECRET")); value != "" {
		return value
	}
	return defaultJWTSecret
}

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			c.Abort()
			return
		}
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid Authorization format"})
			c.Abort()
			return
		}
		userID, err := ParseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}
		c.Set(userContextKey, userID)
		c.Next()
	}
}

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
