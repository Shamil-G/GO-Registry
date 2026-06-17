// --- DB Metrics ---

package metrics

import (
	"context"
	"database/sql"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// --- DB Metrics ---
var (
	DBBuckets = prometheus.ExponentialBuckets(10, 2, 15) // 10µs → ~160ms

	DBSelectDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_select_duration_ms",
			Help:    "DB SELECT latency (ms)",
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
			Name:    "db_sp_duration_ms",
			Help:    "Stored procedure latency (ms)",
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
)

func StartDBPoolMetrics(ctx context.Context, db *sql.DB) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := db.Stats()
				DBPoolOpen.Set(float64(stats.OpenConnections))
				DBPoolInUse.Set(float64(stats.InUse))
				DBPoolIdle.Set(float64(stats.Idle))
			}
		}
	}()
}

func db_init() {
	prometheus.MustRegister(
		DBSelectDuration,
		DBSelectTotal,
		DBSelectErrors,
		DBSPDuration,
		DBSPTotal,
		DBSPErrors,
		DBPoolOpen,
		DBPoolInUse,
		DBPoolIdle,
	)
}
