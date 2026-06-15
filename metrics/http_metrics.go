// --- HTTP Metrics ---
package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	HTTPBuckets = prometheus.ExponentialBuckets(1, 2, 12) // 1ms → ~2s

	HttpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path"},
	)

	HttpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_ms",
			Help:    "HTTP request latency in milliseconds",
			Buckets: HTTPBuckets,
		},
		[]string{"method", "path"},
	)

	HttpErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "Total number of HTTP errors",
		},
		[]string{"method", "path", "code"},
	)
)

func http_init() {
	prometheus.MustRegister(
		HttpRequests,
		HttpRequestDuration,
		HttpErrors,
	)
}
