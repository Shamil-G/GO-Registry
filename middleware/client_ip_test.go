package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"gusseynov/GO-Registry/config"
)

// Инцидент 25.08.2026: после правки nginx в ключ сессии приехало
// "192.168.1.82:8090" — адрес С ПОРТОМ, то есть в IP-заголовок попал
// $http_host. Плюс проверяем сам класс подделки через X-Forwarded-For.
func TestGetClientIP(t *testing.T) {
	saved := config.TrustedProxies
	t.Cleanup(func() { config.TrustedProxies = saved })
	config.TrustedProxies = []string{"127.0.0.1", "::1"}

	cases := []struct {
		name, remote string
		headers      map[string]string
		want         string
	}{
		{
			"прокси на localhost, X-Real-IP — основной источник",
			"127.0.0.1:5090", map[string]string{"X-Real-IP": "192.168.5.59"}, "192.168.5.59",
		},
		{
			"X-Forwarded-For: берём ПОСЛЕДНИЙ элемент, левое прислал клиент",
			"127.0.0.1:5090", map[string]string{"X-Forwarded-For": "10.0.0.1, 192.168.5.59"}, "192.168.5.59",
		},
		{
			"адрес с портом в заголовке отбрасывается (та самая опечатка в nginx)",
			"127.0.0.1:5090",
			map[string]string{"X-Real-IP": "192.168.1.82:8090", "X-Forwarded-For": "192.168.5.59"},
			"192.168.5.59",
		},
		{
			"имя хоста в заголовке отбрасывается, остаётся адрес сокета",
			"127.0.0.1:5090", map[string]string{"X-Real-IP": "srm.gfss.kz"}, "127.0.0.1",
		},
		{
			"прямое обращение мимо прокси — заголовкам не верим",
			"192.168.5.85:41234", map[string]string{"X-Real-IP": "192.168.5.59"}, "192.168.5.85",
		},
		{
			"IPv6-петля приводится к 127.0.0.1",
			"[::1]:5090", nil, "127.0.0.1",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = c.remote
			for k, v := range c.headers {
				r.Header.Set(k, v)
			}
			if got := GetClientIP(r); got != c.want {
				t.Errorf("GetClientIP() = %q, ожидалось %q", got, c.want)
			}
		})
	}
}

// Отдельный случай: в конфиге nginx два `proxy_set_header X-Forwarded-For`
// (сломанный с $http_host и правильный с $proxy_add_x_forwarded_for). nginx
// шлёт ОБА заголовка, причём сломанный первым — проверено на живом nginx
// 25.08.2026. Header.Get вернул бы именно его, поэтому в коде Header.Values.
func TestGetClientIPПриДублеXForwardedFor(t *testing.T) {
	saved := config.TrustedProxies
	t.Cleanup(func() { config.TrustedProxies = saved })
	config.TrustedProxies = []string{"127.0.0.1", "::1"}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "127.0.0.1:5090"
	r.Header.Add("X-Forwarded-For", "192.168.1.82:8090")      // мусор из $http_host
	r.Header.Add("X-Forwarded-For", "10.9.9.9, 192.168.5.59") // подделка клиента + адрес от прокси

	if got := GetClientIP(r); got != "192.168.5.59" {
		t.Errorf("GetClientIP() = %q, ожидалось 192.168.5.59", got)
	}
}
