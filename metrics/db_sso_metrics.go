// metrics/db_sso_metrics.go

package metrics

import "github.com/prometheus/client_golang/prometheus"

var DBBuckets = []float64{
	10, 20, 40, 80,
	160, 320, 640,
	1000, 2000, 4000,
	8000, 16000, 32000,
	64000, 128000, 256000,
	512000, 1024000,
}

var SSOBuckets = []float64{
	10, 20, 40, 80,
	160, 320, 640,
	1000, 2000, 4000,
	8000, 750000, 2058000,
}

var (
	DBSelectDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_select_duration_us",
			Help:    "DB SELECT latency in micro-seconds",
			Buckets: DBBuckets,
		},
		[]string{"query"},
	)

	DBSelectTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_select_total",
			Help: "Total number of DB SELECT operations",
		},
		[]string{"query"},
	)

	DBSelectErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_select_errors_total",
			Help: "Total number of DB SELECT errors",
		},
		[]string{"query", "error"},
	)

	DBSPDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_sp_duration_us",
			Help:    "Stored procedure latency in micro-seconds",
			Buckets: DBBuckets,
		},
		[]string{"procedure"},
	)

	DBSPTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_sp_total",
			Help: "Total number of stored procedure calls",
		},
		[]string{"procedure"},
	)

	DBSPErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_sp_errors_total",
			Help: "Total number of stored procedure errors",
		},
		[]string{"procedure", "error"},
	)

	SSORequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sso_request_duration_us",
			Help:    "SSO request latency in micro-seconds",
			Buckets: SSOBuckets,
		},
		[]string{"endpoint", "code"},
	)

	SSORequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "sso_requests_total",
			Help: "Total SSO requests",
		},
		[]string{"endpoint", "code"},
	)

	DBPoolOpen = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_pool_open_connections",
			Help: "Number of open DB connections",
		},
	)

	DBPoolInUse = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_pool_in_use_connections",
			Help: "Number of DB connections currently in use",
		},
	)

	DBPoolIdle = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_pool_idle_connections",
			Help: "Number of idle DB connections",
		},
	)

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
			Buckets: prometheus.ExponentialBuckets(1, 2, 12),
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

func Init() {
	prometheus.MustRegister(
		DBSelectDuration,
		DBSelectTotal,
		DBSelectErrors,
		DBSPDuration,
		DBSPTotal,
		DBSPErrors,
		SSORequestDuration,
		SSORequestTotal,
		HttpRequests,
		HttpRequestDuration,
		HttpErrors,
		DBPoolOpen,
		DBPoolInUse,
		DBPoolIdle,
	)
}
