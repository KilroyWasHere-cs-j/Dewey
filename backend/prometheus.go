package main
//
// import (
// 	"net/http"
//   "github.com/prometheus/client_golang/prometheus"
// 	"time"
//   "github.com/gin-gonic/gin"
// )
//
// var (
//     httpRequestsTotal = prometheus.NewCounterVec(
//         prometheus.CounterOpts{
//             Name: "http_requests_total",
//             Help: "Total HTTP requests",
//         },
//         []string{"path", "method", "status"},
//     )
//
//     httpRequestDuration = prometheus.NewHistogramVec(
//         prometheus.HistogramOpts{
//             Name:    "http_request_duration_seconds",
//             Help:    "HTTP request duration",
//             Buckets: prometheus.DefBuckets,
//         },
//         []string{"path", "method"},
//     )
// )
//
// func init() {
//     prometheus.MustRegister(httpRequestsTotal)
//     prometheus.MustRegister(httpRequestDuration)
// }
//
// func PrometheusMiddleware() gin.HandlerFunc {
//     return func(c *gin.Context) {
//         start := time.Now()
//
//         c.Next()
//
//         duration := time.Since(start).Seconds()
//         path := c.FullPath()
//         if path == "" {
//             path = "unknown"
//         }
//
//         status := c.Writer.Status()
//
//         httpRequestsTotal.WithLabelValues(path, c.Request.Method, 
//             http.StatusText(status)).Inc()
//
//         httpRequestDuration.WithLabelValues(path, c.Request.Method).
//             Observe(duration)
//     }
// }
