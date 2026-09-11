package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gusseynov/GO-Registry/config"
)

// setupClientIP — доверенный прокси только локальный nginx.
func setupClientIP(t *testing.T, isProd bool) {
	t.Helper()
	savedProxies, savedCfg := config.TrustedProxies, config.Cfg
	t.Cleanup(func() { config.TrustedProxies, config.Cfg = savedProxies, savedCfg })

	config.TrustedProxies = []string{"127.0.0.1", "::1"}
	config.Cfg = &config.Config{IsProd: isProd}
}

// Ключ сессии SSO — цепочка X-Forwarded-For, как её собрал nginx. Формат
// обязан совпадать во всех приложениях (Registry, Court, SRM, SSO), поэтому
// проверяется точная строка, включая пробелы.
//
// 10.09.2026 ключ заменили на X-Real-IP, и все пользователи корпоративного
// прокси 192.168.1.12 оказались в одной сессии. Случай «через прокси» ниже —
// ровно та регрессия.
func TestGetClientIP(t *testing.T) {
	setupClientIP(t, true)

	const nginx = "127.0.0.1:5090"

	cases := []struct {
		name   string
		remote string
		xff    []string // каждый элемент — отдельный заголовок X-Forwarded-For
		realIP string
		want   string
	}{
		{
			name:   "напрямую: nginx создал заголовок сам",
			remote: nginx, xff: []string{"192.168.5.236"}, realIP: "192.168.5.236",
			want: "192.168.5.236",
		},
		{
			name:   "через корпоративный прокси: составной ключ, X-Real-IP (адрес прокси) не используется",
			remote: nginx, xff: []string{"192.168.5.236, 192.168.1.12"}, realIP: "192.168.1.12",
			want: "192.168.5.236, 192.168.1.12",
		},
		{
			name:   "разделитель приводится к формату nginx",
			remote: nginx, xff: []string{"192.168.5.236 ,192.168.1.12"}, realIP: "192.168.1.12",
			want: "192.168.5.236, 192.168.1.12",
		},
		{
			name:   "мусор клиента слева выбрасывается",
			remote: nginx, xff: []string{"abc, 192.168.5.236"}, realIP: "192.168.5.236",
			want: "192.168.5.236",
		},
		{
			name:   "unknown в цепочке от прокси выбрасывается, остальное остаётся",
			remote: nginx, xff: []string{"unknown, 192.168.5.236, 192.168.1.12"}, realIP: "192.168.1.12",
			want: "192.168.5.236, 192.168.1.12",
		},
		{
			// Клиент приписал слева чужой адрес — ключ стал новым, а не чужим:
			// у владельца 192.168.5.59 ключ "192.168.5.59".
			name:   "подделка слева даёт новый ключ, а не чужой",
			remote: nginx, xff: []string{"192.168.5.59, 192.168.5.236"}, realIP: "192.168.5.236",
			want: "192.168.5.59, 192.168.5.236",
		},
		{
			// Инцидент 25.08.2026: две строки proxy_set_header, сломанная первой.
			name:   "дубль заголовка, сломанный первым — берём последний",
			remote: nginx, xff: []string{"192.168.1.82:8090", "192.168.5.59"},
			want: "192.168.5.59",
		},
		{
			name:   "дубль заголовка, сломанный последним — берём предыдущий с адресами",
			remote: nginx, xff: []string{"192.168.5.59", "192.168.1.82:8090"},
			want: "192.168.5.59",
		},
		{
			name:   "X-Forwarded-For сломан (адрес с портом) — аварийно X-Real-IP",
			remote: nginx, xff: []string{"192.168.1.82:8090"}, realIP: "192.168.5.59",
			want: "192.168.5.59",
		},
		{
			name:   "X-Forwarded-For нет — аварийно X-Real-IP",
			remote: nginx, realIP: "192.168.5.59",
			want: "192.168.5.59",
		},
		{
			name:   "ни одного адреса — пустой ключ, а не 127.0.0.1 на всех",
			remote: nginx, realIP: "srm.gfss.kz",
			want: "",
		},
		{
			name:   "заголовков нет вовсе — пустой ключ",
			remote: nginx,
			want:   "",
		},
		{
			name:   "прямое обращение мимо nginx — заголовкам не верим",
			remote: "192.168.5.85:41234", xff: []string{"192.168.5.59"}, realIP: "192.168.5.59",
			want: "192.168.5.85",
		},
		{
			name:   "IPv6-петля в цепочке приводится к 127.0.0.1",
			remote: "[::1]:5090", xff: []string{"::1"},
			want: "127.0.0.1",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = c.remote
			for _, v := range c.xff {
				r.Header.Add("X-Forwarded-For", v)
			}
			if c.realIP != "" {
				r.Header.Set("X-Real-IP", c.realIP)
			}
			if got := GetClientIP(r); got != c.want {
				t.Errorf("GetClientIP() = %q, ожидалось %q", got, c.want)
			}
		})
	}
}

// Разработка: сервис на 127.0.0.1 без nginx, заголовков нет — ключом остаётся
// адрес сокета, иначе локально не войти.
func TestGetClientIPРазработка(t *testing.T) {
	setupClientIP(t, false)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "[::1]:5090"

	if got := GetClientIP(r); got != "127.0.0.1" {
		t.Errorf("GetClientIP() = %q, ожидалось 127.0.0.1", got)
	}
}
