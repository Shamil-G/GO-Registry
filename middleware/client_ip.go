// middleware/client_ip.go
//
// Определение IP клиента. Это НЕ косметика: сессия SSO привязана к IP-адресу,
// то есть тот, кто может подделать этот адрес, заходит под чужой сессией.
// Перенесено из GO-SSO (middleware/ip.go) 25.08.2026 после инцидента на бою:
// прежняя реализация читала заголовки БЕЗУСЛОВНО и отдавала их в SSO как есть,
// поэтому опечатка в конфиге nginx (в IP-заголовок попал $http_host) привела
// к тому, что ключом сессии стало "192.168.1.82:8090" — адрес С ПОРТОМ.
//
// Две дырки, которые закрывает эта редакция:
//
//  1. Заголовки читались безусловно. Если сервис слушает не только loopback,
//     в него можно постучаться мимо nginx и прислать любой X-Real-IP. Теперь
//     заголовкам верим, ТОЛЬКО если сокет пришёл с адреса из
//     config.TrustedProxies (TRUSTED_PROXIES в .env).
//  2. Из X-Forwarded-For бралась вся строка целиком (а в наивных вариантах —
//     первый элемент). nginx собирает заголовок как $proxy_add_x_forwarded_for,
//     то есть ДОБАВЛЯЕТ свой $remote_addr в конец к тому, что прислал клиент.
//     Клиент шлёт "X-Forwarded-For: <чужой IP>" и получает
//     "<чужой IP>, <свой настоящий>". Первый элемент — ровно подделка.
//     Правильный при этой схеме — ПОСЛЕДНИЙ.
//
// Плюс net.ParseIP на каждое значение из заголовка: мусор не должен
// становиться ключом сессии, и это же ловит опечатку в конфиге прокси.
package middleware

import (
	"net"
	"net/http"
	"strings"

	"gusseynov/GO-Registry/config"
)

// GetClientIP — адрес клиента с учётом доверенного прокси.
func GetClientIP(r *http.Request) string {
	peer := remoteHost(r)

	// Запрос пришёл не от доверенного прокси — это прямое обращение.
	// Заголовки в нём произвольные, верим только адресу сокета.
	if !config.IsTrustedProxy(peer) {
		return peer
	}

	// X-Real-IP: nginx перезаписывает его своим $remote_addr, подделать
	// нельзя. Основной источник.
	if ip := normalizeIP(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}

	// X-Forwarded-For: последний элемент добавлен прокси и равен адресу,
	// с которого прокси принял соединение. Всё, что левее, прислал клиент.
	//
	// Values, а не Get: заголовок может прийти НЕСКОЛЬКО раз. Если в конфиге
	// nginx оказалось два `proxy_set_header X-Forwarded-For ...` (у нас так и
	// было — один со сломанным $http_host), nginx отправляет ОБА заголовка, а
	// Header.Get вернул бы только первый, то есть как раз сломанный. Идём с
	// конца: последнее значение последнего заголовка ближе всего к правде.
	// Признак такого конфига в nginx error.log — предупреждение
	// "could not build optimal proxy_headers_hash".
	values := r.Header.Values("X-Forwarded-For")
	for v := len(values) - 1; v >= 0; v-- {
		parts := strings.Split(values[v], ",")
		for i := len(parts) - 1; i >= 0; i-- {
			if ip := normalizeIP(parts[i]); ip != "" {
				return ip
			}
		}
	}

	return peer
}

// remoteHost — адрес сокета без порта.
func remoteHost(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr без порта (бывает в тестах) — берём как есть.
		return normalizeLoopback(strings.TrimSpace(r.RemoteAddr))
	}
	return normalizeLoopback(host)
}

// normalizeIP — обрезает пробелы и проверяет, что это действительно адрес.
// Мусор из заголовка ("192.168.1.82:8090", имя хоста, пустая строка) не должен
// становиться ключом сессии.
func normalizeIP(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || net.ParseIP(v) == nil {
		return ""
	}
	return normalizeLoopback(v)
}

// normalizeLoopback — IPv6-петля приводится к IPv4-виду: SSO и денормализованные
// поля в Oracle везде ждут "127.0.0.1".
func normalizeLoopback(ip string) string {
	if ip == "::1" {
		return "127.0.0.1"
	}
	return ip
}
