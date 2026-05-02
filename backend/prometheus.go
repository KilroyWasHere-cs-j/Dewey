package main

import (
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/gin-gonic/gin"
)

// appName is set from the APP_NAME env var or falls back to a sensible default.
var appName = func() string {
	if v := os.Getenv("APP_NAME"); v != "" {
		return v
	}
	return "cross_doc_tool"
}()

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "http_requests_total",
			Help:        "Total HTTP requests",
			ConstLabels: prometheus.Labels{"app": appName},
		},
		[]string{"path", "method", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "http_request_duration_seconds",
			Help:        "HTTP request duration",
			Buckets:     prometheus.DefBuckets,
			ConstLabels: prometheus.Labels{"app": appName},
		},
		[]string{"path", "method"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
}

func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		status := c.Writer.Status()

		httpRequestsTotal.WithLabelValues(path, c.Request.Method, http.StatusText(status)).Inc()

		httpRequestDuration.WithLabelValues(path, c.Request.Method).Observe(duration)
	}
}
