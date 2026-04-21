package middleware

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const userContextKey = "user_id"

func BuildToken(userID uint) string {
	payload := "u:" + strconv.FormatUint(uint64(userID), 10)
	return base64.StdEncoding.EncodeToString([]byte(payload))
}

func ParseToken(token string) (uint, error) {
	decoded, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return 0, err
	}
	parts := strings.Split(string(decoded), ":")
	if len(parts) != 2 || parts[0] != "u" {
		return 0, errors.New("invalid token")
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
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
