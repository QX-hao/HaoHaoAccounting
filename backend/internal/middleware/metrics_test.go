package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func TestHTTPMetricsUsesRoutePatternLabels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := prometheus.NewRegistry()
	metrics := NewHTTPMetrics(registry)
	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/api/v1/accounts/:id", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/accounts/123?token=secret", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	gathered, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	wantLabels := map[string]string{"method": "GET", "route": "/api/v1/accounts/:id", "status": "204"}
	if !hasMetricWithLabels(gathered, "haohao_http_requests_total", wantLabels) {
		t.Fatalf("metrics = %s, missing request counter labels %#v", metricFamiliesText(gathered), wantLabels)
	}
	if !hasMetricWithLabels(gathered, "haohao_http_request_duration_seconds", wantLabels) {
		t.Fatalf("metrics = %s, missing duration histogram labels %#v", metricFamiliesText(gathered), wantLabels)
	}
	metricsText := metricFamiliesText(gathered)
	for _, leaked := range []string{"123", "token=secret"} {
		if strings.Contains(metricsText, leaked) {
			t.Fatalf("metrics leaked raw request data %q: %s", leaked, metricsText)
		}
	}
}

func TestHTTPMetricsGroupsUnmatchedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := prometheus.NewRegistry()
	metrics := NewHTTPMetrics(registry)
	router := gin.New()
	router.Use(metrics.Middleware())

	req := httptest.NewRequest(http.MethodGet, "/missing/path?token=secret", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	gathered, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	metricsText := metricFamiliesText(gathered)
	wantLabels := map[string]string{"method": "GET", "route": "unmatched", "status": "404"}
	if !hasMetricWithLabels(gathered, "haohao_http_requests_total", wantLabels) {
		t.Fatalf("metrics = %s, missing unmatched route labels %#v", metricsText, wantLabels)
	}
	if strings.Contains(metricsText, "/missing/path") || strings.Contains(metricsText, "token=secret") {
		t.Fatalf("metrics leaked unmatched URL: %s", metricsText)
	}
}

func TestHTTPMetricsNormalizesUnknownMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := prometheus.NewRegistry()
	metrics := NewHTTPMetrics(registry)
	router := gin.New()
	router.Use(metrics.Middleware())
	router.Handle("PROPFIND", "/api/v1/accounts", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest("PROPFIND", "/api/v1/accounts", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	gathered, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	metricsText := metricFamiliesText(gathered)
	wantLabels := map[string]string{"method": "UNKNOWN", "route": "/api/v1/accounts", "status": "204"}
	if !hasMetricWithLabels(gathered, "haohao_http_requests_total", wantLabels) {
		t.Fatalf("metrics = %s, missing normalized method labels %#v", metricsText, wantLabels)
	}
	if strings.Contains(metricsText, "PROPFIND") {
		t.Fatalf("metrics leaked raw unknown method: %s", metricsText)
	}
}

func hasMetricWithLabels(families []*dto.MetricFamily, name string, labels map[string]string) bool {
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if metricHasLabels(metric, labels) {
				return true
			}
		}
	}
	return false
}

func metricHasLabels(metric *dto.Metric, labels map[string]string) bool {
	got := make(map[string]string, len(metric.GetLabel()))
	for _, label := range metric.GetLabel() {
		got[label.GetName()] = label.GetValue()
	}
	for name, want := range labels {
		if got[name] != want {
			return false
		}
	}
	return true
}

func metricFamiliesText(families []*dto.MetricFamily) string {
	var builder strings.Builder
	for _, family := range families {
		builder.WriteString(family.String())
	}
	return builder.String()
}
