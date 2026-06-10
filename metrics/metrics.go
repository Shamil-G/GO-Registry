// metrics/metrics.go

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	CacheSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "memory_cache_size",
		Help: "Number of active keys in MemoryRepo",
	})

	MemoryGetTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "memory_get_total",
		Help: "Total number of Get() calls",
	})

	MemoryGetHit = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "memory_get_hit",
		Help: "Cache hits in MemoryRepo",
	})

	MemoryGetMiss = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "memory_get_miss",
		Help: "Cache misses in MemoryRepo",
	})

	MemoryGetExpired = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "memory_get_expired",
		Help: "Expired keys detected in Get()",
	})

	MemorySaveTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "memory_save_total",
		Help: "Total number of Save() calls",
	})

	MemoryListTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "memory_list_total",
		Help: "Total number of List() calls",
	})

	HttpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path"},
	)

	FastDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "api_fast_duration_ms",
			Help: "Fast HTTP endpoints",
			Buckets: []float64{
				0.05, 0.075, 0.1,
				0.25, 0.5, 0.75, 1, 2, 5, 10, 25,
			},
		},
		[]string{"method", "path"},
	)

	MiddleDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "api_middle_duration_ms",
			Help: "Middle HTTP endpoints",
			Buckets: []float64{
				1, 3, 5, 10, 25, 50, 100, 250, 500,
			},
		},
		[]string{"method", "path"},
	)

	SlowDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "api_slow_duration_ms",
			Help: "Slow HTTP endpoints",
			Buckets: []float64{
				10, 25, 50, 75, 100, 150, 250, 500, 750, 1000, 1500, 2000, 2500,
			},
		},
		[]string{"method", "path"},
	)
)

func Init() {
	// Включаем стандартные Go runtime метрики
	// prometheus.MustRegister(collectors.NewGoCollector())
	// prometheus.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	// Histograms
	prometheus.MustRegister(FastDuration)
	prometheus.MustRegister(MiddleDuration)
	prometheus.MustRegister(SlowDuration)

	prometheus.MustRegister(CacheSize)
	prometheus.MustRegister(MemoryGetTotal)
	prometheus.MustRegister(MemoryGetHit)
	prometheus.MustRegister(MemoryGetMiss)
	prometheus.MustRegister(MemoryGetExpired)
	prometheus.MustRegister(MemorySaveTotal)
	prometheus.MustRegister(MemoryListTotal)
	prometheus.MustRegister(HttpRequests)
}
