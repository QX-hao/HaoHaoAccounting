package middleware

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

type HTTPMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewHTTPMetrics 注册低基数 HTTP 指标，标签只包含方法、Gin 路由模板和状态码。
func NewHTTPMetrics(registry *prometheus.Registry) *HTTPMetrics {
	metrics := &HTTPMetrics{
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "haohao_http_requests_total",
			Help: "Total HTTP requests handled by method, route, and status.",
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "haohao_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds by method, route, and status.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"method", "route", "status"}),
	}
	registry.MustRegister(metrics.requests, metrics.duration)
	return metrics
}

// Middleware 在请求结束后记录指标；未匹配路由统一归为 unmatched，避免原始 URL 进入 label。
func (metrics *HTTPMetrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()

		method := normalizedMetricMethod(c.Request.Method)
		status := normalizedMetricStatus(c.Writer.Status())
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		metrics.requests.WithLabelValues(method, route, status).Inc()
		metrics.duration.WithLabelValues(method, route, status).Observe(time.Since(started).Seconds())
	}
}

func normalizedMetricMethod(method string) string {
	clean := strings.ToUpper(strings.TrimSpace(method))
	switch clean {
	case "GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS", "TRACE":
		return clean
	default:
		return "UNKNOWN"
	}
}

func normalizedMetricStatus(status int) string {
	if status < 100 || status > 599 {
		return "000"
	}
	return strconv.Itoa(status)
}
