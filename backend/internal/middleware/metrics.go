package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

type HTTPMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

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

func (metrics *HTTPMetrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()

		status := strconv.Itoa(c.Writer.Status())
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		metrics.requests.WithLabelValues(c.Request.Method, route, status).Inc()
		metrics.duration.WithLabelValues(c.Request.Method, route, status).Observe(time.Since(started).Seconds())
	}
}
