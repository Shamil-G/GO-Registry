package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	DBSelectDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_select_duration_ms",
			Help:    "DB SELECT latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 12),
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
			Name:    "db_sp_duration_ms",
			Help:    "Stored procedure latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 12),
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
			Name:    "sso_request_duration_ms",
			Help:    "SSO request latency in milliseconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 12),
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
)

func MustRegister() {
	prometheus.MustRegister(
		DBSelectDuration,
		DBSelectTotal,
		DBSelectErrors,
		DBSPDuration,
		DBSPTotal,
		DBSPErrors,
		SSORequestDuration,
		SSORequestTotal,
	)
}
