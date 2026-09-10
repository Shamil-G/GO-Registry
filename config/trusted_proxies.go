// config/trusted_proxies.go
//
// TrustedProxies — адреса, чьим заголовкам X-Real-IP / X-Forwarded-For можно
// верить. Перенесено из GO-SSO 25.08.2026, см. middleware/client_ip.go.
//
// На бою перед сервисом стоит nginx, который проксирует на localhost, то есть
// для проксированных запросов сервис видит RemoteAddr = 127.0.0.1. Всё, что
// приходит НЕ с этих адресов, — стук напрямую в порт мимо nginx, и заголовкам
// такого клиента верить нельзя.
//
// Переопределяется переменной TRUSTED_PROXIES в .env (через запятую).
// ЗАМЕНЯЕТ список по умолчанию целиком, а не дополняет: список тех, кто может
// назвать чужой IP от имени клиента, должен быть точным и коротким.
package config

import (
	"log/slog"
	"net"
	"os"
	"slices"
	"strings"
)

var TrustedProxies = []string{"127.0.0.1", "::1"}

// LoadTrustedProxies читает TRUSTED_PROXIES из .env.
// Вызывается из LoadConfig, то есть уже ПОСЛЕ godotenv.Load().
func LoadTrustedProxies() {
	env := os.Getenv("TRUSTED_PROXIES")
	if env == "" {
		slog.Info("TRUSTED_PROXIES не задан — доверяем заголовкам только от локального прокси",
			"proxies", TrustedProxies)
		return
	}

	list := make([]string, 0, 4)
	for _, p := range strings.Split(env, ",") {
		cleanIP := strings.TrimSpace(p)
		if cleanIP == "" {
			continue
		}
		if net.ParseIP(cleanIP) == nil {
			slog.Warn("TRUSTED_PROXIES: не похоже на IP-адрес, строка пропущена", "value", cleanIP)
			continue
		}
		if !slices.Contains(list, cleanIP) {
			list = append(list, cleanIP)
		}
	}

	if len(list) == 0 {
		slog.Warn("TRUSTED_PROXIES задан, но ни одного корректного адреса не разобрано — оставляю значение по умолчанию",
			"proxies", TrustedProxies)
		return
	}

	TrustedProxies = list
	slog.Info("TRUSTED_PROXIES загружен из .env", "proxies", TrustedProxies)
}

// IsTrustedProxy — можно ли верить заголовкам X-Real-IP / X-Forwarded-For,
// пришедшим с этого адреса.
func IsTrustedProxy(ip string) bool {
	return slices.Contains(TrustedProxies, strings.TrimSpace(ip))
}
