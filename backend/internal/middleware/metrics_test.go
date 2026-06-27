package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	for _, leaked := range []string{"123", "token=secret"} {
		if metricLabelsContain(gathered, leaked) {
			t.Fatalf("metrics leaked raw request label %q: %s", leaked, metricFamiliesText(gathered))
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
	if metricLabelsContain(gathered, "/missing/path") || metricLabelsContain(gathered, "token=secret") {
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
	if metricLabelsContain(gathered, "PROPFIND") {
		t.Fatalf("metrics leaked raw unknown method: %s", metricsText)
	}
}

func TestHTTPMetricsTracksInFlightRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registry := prometheus.NewRegistry()
	metrics := NewHTTPMetrics(registry)
	router := gin.New()
	router.Use(metrics.Middleware())

	entered := make(chan struct{})
	release := make(chan struct{})
	router.GET("/api/v1/slow", func(c *gin.Context) {
		close(entered)
		<-release
		c.Status(http.StatusNoContent)
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/slow", nil)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		if resp.Code != http.StatusNoContent {
			t.Errorf("status = %d, body = %s", resp.Code, resp.Body.String())
		}
	}()

	<-entered
	if got := metricGaugeValue(t, registry, "haohao_http_requests_in_flight"); got != 1 {
		t.Fatalf("in-flight during request = %v, want 1", got)
	}

	close(release)
	wg.Wait()
	if got := metricGaugeValue(t, registry, "haohao_http_requests_in_flight"); got != 0 {
		t.Fatalf("in-flight after request = %v, want 0", got)
	}
}

func TestNormalizedMetricStatusBoundsHTTPStatusLabels(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{status: 99, want: "000"},
		{status: 100, want: "100"},
		{status: 204, want: "204"},
		{status: 599, want: "599"},
		{status: 600, want: "000"},
		{status: 0, want: "000"},
	}

	for _, tt := range tests {
		if got := normalizedMetricStatus(tt.status); got != tt.want {
			t.Fatalf("normalizedMetricStatus(%d) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func metricGaugeValue(t *testing.T, registry *prometheus.Registry, name string) float64 {
	t.Helper()

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		metrics := family.GetMetric()
		if len(metrics) != 1 || metrics[0].GetGauge() == nil {
			t.Fatalf("%s metrics = %s, want exactly one gauge sample", name, metricFamiliesText(families))
		}
		return metrics[0].GetGauge().GetValue()
	}
	t.Fatalf("metrics = %s, missing gauge %s", metricFamiliesText(families), name)
	return 0
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

func metricLabelsContain(families []*dto.MetricFamily, needle string) bool {
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if strings.Contains(label.GetValue(), needle) {
					return true
				}
			}
		}
	}
	return false
}

func metricFamiliesText(families []*dto.MetricFamily) string {
	var builder strings.Builder
	for _, family := range families {
		builder.WriteString(family.String())
	}
	return builder.String()
}
