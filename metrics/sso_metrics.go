// --- SSO Metrics ---
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	SSOBuckets = prometheus.ExponentialBuckets(100, 2, 12)

	SSORequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "sso_request_duration_us",
			Help:    "SSO request latency (microseconds)",
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
)

func sso_init() {
	prometheus.MustRegister(
		SSORequestDuration,
		SSORequestTotal,
	)
}
