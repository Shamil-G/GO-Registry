package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func SlogLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", float64(time.Since(start).Microseconds())/1000,
			"ip", r.RemoteAddr,
		)
	})
}
