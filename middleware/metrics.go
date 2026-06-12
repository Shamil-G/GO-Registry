package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"gusseynov/GO-Registry/metrics"
)

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Выполняем запрос
		next.ServeHTTP(w, r)

		// Длительность
		durationMs := float64(time.Since(start).Microseconds())

		// Счётчик запросов
		metrics.HttpRequests.WithLabelValues(r.Method, r.URL.Path).Inc()

		// Гистограмма длительности
		metrics.HttpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(durationMs)

		// Лог
		slog.Debug("request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_us", durationMs,
			"ip", r.RemoteAddr,
		)
	})
}
