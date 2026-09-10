package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gusseynov/GO-Registry/config"
)

// setupMetricsCfg подменяет конфиг на время теста.
func setupMetricsCfg(t *testing.T, allowed []string, proxies []string) {
	t.Helper()

	oldCfg, oldProxies := config.Cfg, config.TrustedProxies
	t.Cleanup(func() { config.Cfg, config.TrustedProxies = oldCfg, oldProxies })

	config.Cfg = &config.Config{MetricsAllowedIPs: allowed}
	config.TrustedProxies = proxies
}

// okHandler — заглушка вместо promhttp.Handler().
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("metrics"))
})

func TestRequireMetricsAccess(t *testing.T) {
	cases := []struct {
		name       string
		allowed    []string
		proxies    []string
		remoteAddr string
		realIP     string // X-Real-IP, как его ставит nginx
		wantCode   int
	}{
		{
			name:       "список пуст — пускаем всех, как было раньше",
			allowed:    nil,
			remoteAddr: "10.20.30.40:5555",
			wantCode:   http.StatusOK,
		},
		{
			name:       "прямое обращение с разрешённого адреса",
			allowed:    []string{"10.20.30.40"},
			remoteAddr: "10.20.30.40:5555",
			wantCode:   http.StatusOK,
		},
		{
			name:       "прямое обращение с чужого адреса",
			allowed:    []string{"10.20.30.40"},
			remoteAddr: "10.20.30.41:5555",
			wantCode:   http.StatusForbidden,
		},
		{
			// Так ходит Prometheus: цель 192.168.1.34:80, то есть через nginx.
			// Адрес сокета — локальный, настоящий приходит в X-Real-IP.
			name:       "через доверенный прокси, разрешённый X-Real-IP",
			allowed:    []string{"192.168.20.33"},
			proxies:    []string{"127.0.0.1"},
			remoteAddr: "127.0.0.1:41234",
			realIP:     "192.168.20.33",
			wantCode:   http.StatusOK,
		},
		{
			name:       "через доверенный прокси, чужой X-Real-IP",
			allowed:    []string{"192.168.20.33"},
			proxies:    []string{"127.0.0.1"},
			remoteAddr: "127.0.0.1:41234",
			realIP:     "10.20.30.41",
			wantCode:   http.StatusForbidden,
		},
		{
			// Заголовку от НЕдоверенного адреса верить нельзя: иначе любой
			// пришлёт X-Real-IP разрешённого и заберёт метрики.
			name:       "подделка X-Real-IP мимо прокси",
			allowed:    []string{"192.168.20.33"},
			proxies:    []string{"127.0.0.1"},
			remoteAddr: "10.20.30.41:5555",
			realIP:     "192.168.20.33",
			wantCode:   http.StatusForbidden,
		},
		{
			// Ловушка конфигурации: nginx проксирует, но X-Real-IP не ставит.
			// Тогда сервис видит адрес самого прокси — и если его нет в списке,
			// закрывается всё, включая Prometheus.
			name:       "прокси без X-Real-IP",
			allowed:    []string{"192.168.20.33"},
			proxies:    []string{"127.0.0.1"},
			remoteAddr: "127.0.0.1:41234",
			wantCode:   http.StatusForbidden,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setupMetricsCfg(t, c.allowed, c.proxies)

			r := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			r.RemoteAddr = c.remoteAddr
			if c.realIP != "" {
				r.Header.Set("X-Real-IP", c.realIP)
			}

			w := httptest.NewRecorder()
			RequireMetricsAccess(okHandler).ServeHTTP(w, r)

			if w.Code != c.wantCode {
				t.Errorf("код %d, ожидался %d; тело: %s", w.Code, c.wantCode, w.Body.String())
			}
		})
	}
}

// TestMetricsDeniedLogThrottle — на бою Prometheus стучится каждые 5 секунд, и
// без подавления повторов лог получает 17 тысяч одинаковых строк в сутки.
// Проверяем, что со второго отказа с того же адреса запись не пишется, а
// счётчик подавленных растёт.
func TestMetricsDeniedLogThrottle(t *testing.T) {
	t.Cleanup(func() {
		deniedMu.Lock()
		deniedLog = make(map[string]*deniedEntry)
		deniedMu.Unlock()
	})

	deniedMu.Lock()
	deniedLog = make(map[string]*deniedEntry)
	deniedMu.Unlock()

	const ip = "192.168.1.57"
	allowed := []string{"192.168.5.59"}

	// Первый отказ пишется всегда.
	logMetricsDenied(ip, allowed)

	deniedMu.Lock()
	first := deniedLog[ip].last
	deniedMu.Unlock()
	if first.IsZero() {
		t.Fatal("первый отказ не отмечен как записанный")
	}

	// Следующие 100 — подавляются.
	for i := 0; i < 100; i++ {
		logMetricsDenied(ip, allowed)
	}

	deniedMu.Lock()
	e := deniedLog[ip]
	suppressed, last := e.suppressed, e.last
	deniedMu.Unlock()

	if suppressed != 100 {
		t.Errorf("подавлено %d повторов, ожидалось 100", suppressed)
	}
	if !last.Equal(first) {
		t.Error("время записи обновилось, хотя строка не писалась")
	}

	// Другой адрес — отдельный счётчик, его подавление первого не касается.
	logMetricsDenied("10.0.0.9", allowed)
	deniedMu.Lock()
	other, ok := deniedLog["10.0.0.9"]
	otherLast := time.Time{}
	if ok {
		otherLast = other.last
	}
	deniedMu.Unlock()
	if otherLast.IsZero() {
		t.Error("отказ с нового адреса не записан")
	}
}
