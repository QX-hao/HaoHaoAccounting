package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		SetNoStore(c.Writer.Header())
		c.Next()
	}
}

func NoStoreAPI(prefix string) gin.HandlerFunc {
	prefix = strings.TrimRight(prefix, "/")
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			SetNoStore(c.Writer.Header())
		}
		c.Next()
	}
}

func SetNoStore(headers http.Header) {
	headers.Set("Cache-Control", "no-store")
	headers.Set("Pragma", "no-cache")
	headers.Set("Expires", "0")
}
