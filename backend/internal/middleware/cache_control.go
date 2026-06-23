package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// NoStore 为业务 API 响应设置禁止缓存头，避免用户数据或错误响应被浏览器、代理复用。
func NoStore() gin.HandlerFunc {
	return func(c *gin.Context) {
		SetNoStore(c.Writer.Header())
		c.Next()
	}
}

// NoStoreAPI 只匹配指定 API 前缀，防止误伤健康检查、静态资源等非业务接口。
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

// SetNoStore 用于登录、账单等敏感 API 响应，要求客户端和中间代理不要存储响应。
func SetNoStore(headers http.Header) {
	headers.Set("Cache-Control", "no-store")
	headers.Set("Pragma", "no-cache")
	headers.Set("Expires", "0")
}

// SetNoCache 用于健康检查等非敏感但必须新鲜的响应，允许存储但复用前必须重新校验。
func SetNoCache(headers http.Header) {
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Pragma", "no-cache")
	headers.Set("Expires", "0")
}
