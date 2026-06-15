package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		SetNoStore(c.Writer.Header())
		c.Next()
	}
}

func SetNoStore(headers http.Header) {
	headers.Set("Cache-Control", "no-store")
	headers.Set("Pragma", "no-cache")
	headers.Set("Expires", "0")
}
