package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"gusseynov/GO-Registry/metrics"
)

func Metrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Пропускаем всё, что не в списке
		cfg, ok := metrics.Endpoints[r.URL.Path]
		if !ok {
			// endpoint не мониторим
			next.ServeHTTP(w, r)
			return
		}

		// Проверяем метод
		if cfg.Method != r.Method {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		// Передаем управление по адресу w
		next.ServeHTTP(w, r)
		// Запрос обработан, получен ответ и изменяем продолжительность!
		duration := float64(time.Since(start).Microseconds()) / 1000.0 // ms
		// Счетчик запросов
		metrics.HttpRequests.WithLabelValues(r.Method, r.URL.Path).Inc()

		// Выбор бакета
		switch cfg.Speed {
		case metrics.FAST:
			metrics.FastDuration.WithLabelValues(cfg.Method, r.URL.Path).Observe(duration)
		case metrics.MIDDLE:
			metrics.MiddleDuration.WithLabelValues(cfg.Method, r.URL.Path).Observe(duration)
		case metrics.SLOW:
			metrics.SlowDuration.WithLabelValues(cfg.Method, r.URL.Path).Observe(duration)
		}

		dur := time.Since(start)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", float64(dur.Microseconds())/1000,
			"ip", r.RemoteAddr,
		)
	})
}
