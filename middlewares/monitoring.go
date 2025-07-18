package middlewares

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "order_service_http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "order_service_http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path", "status"},
	)

	orderOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "order_service_order_operations_total",
			Help: "Total number of order operations",
		},
		[]string{"operation", "status"},
	)
)

// PrometheusMiddleware 收集 Prometheus 指标
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		httpRequestsTotal.WithLabelValues(
			c.Request.Method,
			path,
			status,
		).Inc()

		httpRequestDuration.WithLabelValues(
			c.Request.Method,
			path,
			status,
		).Observe(duration)
	}
}

// RecordOrderOperation 记录订单操作指标
func RecordOrderOperation(operation string, success bool) {
	status := "success"
	if !success {
		status = "error"
	}
	orderOperations.WithLabelValues(operation, status).Inc()
}
