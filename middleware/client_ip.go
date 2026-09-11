// middleware/client_ip.go
//
// Определение ключа сессии клиента. Это НЕ косметика: сессия SSO привязана к
// этому значению (user:sso:<ключ>), то есть тот, кто может его подделать,
// заходит под чужой сессией.
//
// ПРАВИЛО ОДНО ДЛЯ ВСЕХ ПРИЛОЖЕНИЙ (GO-Registry, GO-Court, GO-SRM, GO-SSO,
// GO-Test). Ключ общий: Registry создаёт сессию при входе, Court её проверяет.
// Разойдётся формат хотя бы на пробел — пользователь «потеряется» между
// приложениями. Менять разбор только во всех репозиториях сразу, выкатывать
// вместе и только после явного согласия владельца (скилл go-web-conventions,
// раздел 7).
//
// Ключ — цепочка из X-Forwarded-For, как её собрал nginx
// ($proxy_add_x_forwarded_for = «что пришло» + ", " + $remote_addr):
//
//	рабочая станция → nginx                          "192.168.5.236"
//	рабочая станция → прокси 192.168.1.12 → nginx    "192.168.5.236, 192.168.1.12"
//
// Составной ключ осознанный: людей за одним прокси он разделяет, и по нему
// видно, с какого прокси пришёл человек. 10.09.2026 его заменили на X-Real-IP
// (адрес того, кто подключился к nginx), и все пользователи корпоративного
// прокси оказались в одной сессии. Вернули 11.09.2026.
//
// Почему цепочку нельзя подделать: nginx всегда дописывает в конец настоящий
// адрес подключения. Клиент может добавить что угодно только СЛЕВА, и ключ
// получится новым, ни с чьим не совпадающим.
//
// Защиты, которые нужно сохранять (инцидент 25.08.2026):
//
//  1. Заголовкам верим, только если сокет пришёл с адреса из
//     config.TrustedProxies (TRUSTED_PROXIES в .env). Прямой стук в порт мимо
//     nginx — ключом будет адрес сокета, заголовки игнорируются.
//  2. Каждый элемент цепочки проверяется через net.ParseIP, мусор выбрасывается:
//     однажды в ключ попал "192.168.1.82:8090" — $http_host вместо адреса.
//     Порт НЕ обрезать: адрес с портом в заголовке — признак сломанного
//     конфига, а не адрес клиента.
//
// В nginx НЕ включать realip (set_real_ip_from): он подменяет $remote_addr, и
// цепочка превращается в "192.168.5.236, 192.168.5.236" — адрес прокси теряется.
package middleware

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"gusseynov/GO-Registry/config"
)

// errNoClientIP — ключ сессии не определён, в SSO с ним ходить нельзя.
var errNoClientIP = errors.New("адрес клиента не определён")

// GetClientIP — ключ сессии клиента с учётом доверенного прокси.
// Пустая строка — ключ определить нельзя (сломан конфиг nginx). С пустым
// ключом в SSO не ходят: см. PageContext, LoginPost, LogoutGet.
func GetClientIP(r *http.Request) string {
	peer := remoteHost(r)

	// Запрос пришёл не от доверенного прокси — это прямое обращение.
	// Заголовки в нём произвольные, верим только адресу сокета.
	if !config.IsTrustedProxy(peer) {
		return peer
	}

	if chain := forwardedChain(r); chain != "" {
		return chain
	}

	// Разработка: сервис запущен локально без nginx, браузер ходит прямо на
	// 127.0.0.1:DEVELOP_PORT, заголовков нет, и адрес сокета — это и есть
	// пользователь.
	if config.Cfg != nil && !config.Cfg.IsProd {
		return peer
	}

	// Сюда правильный nginx не доводит: $remote_addr, который он дописывает в
	// X-Forwarded-For, всегда валидный адрес, а мусор клиента бывает только
	// левее него. Значит, сломан сам конфиг — строку proxy_set_header
	// X-Forwarded-For удалили или вписали в неё не ту переменную.
	//
	// X-Real-IP — тот же $remote_addr: прямому пользователю он даёт тот же
	// ключ, что и цепочка, и сессия не теряется. Пользователи прокси получат
	// адрес прокси, то есть общий ключ, — поэтому это аварийный путь, а не
	// рабочий, и о нём пишется в лог.
	if ip := normalizeIP(r.Header.Get("X-Real-IP")); ip != "" {
		logBrokenForwardedFor(r, ip)
		return ip
	}

	// Адрес сокета здесь — сам nginx (127.0.0.1): вернуть его значило бы дать
	// всем пользователям один ключ и одну сессию. Лучше не пустить никого.
	logBrokenForwardedFor(r, "")
	return ""
}

// forwardedChain — цепочка адресов из X-Forwarded-For через ", ", без мусора.
// "" — ни одного валидного адреса: заголовка нет или он сломан.
//
// Values, а не Get: заголовок может прийти НЕСКОЛЬКО раз. Если в конфиге
// nginx оказалось два `proxy_set_header X-Forwarded-For ...` (так было
// 25.08.2026 — один со сломанным $http_host), nginx отправляет оба, причём
// сломанный первым. Идём с конца и берём первый заголовок, в котором есть
// адреса. Признак такого конфига в nginx error.log — предупреждение
// "could not build optimal proxy_headers_hash".
//
// Разделитель ", " — ровно как у nginx. Пробелы из заголовка как есть не
// переносятся: ключ должен совпадать байт в байт во всех приложениях.
func forwardedChain(r *http.Request) string {
	values := r.Header.Values("X-Forwarded-For")
	for v := len(values) - 1; v >= 0; v-- {
		var ips []string
		for _, part := range strings.Split(values[v], ",") {
			if ip := normalizeIP(part); ip != "" {
				ips = append(ips, ip)
			}
		}
		if len(ips) > 0 {
			return strings.Join(ips, ", ")
		}
	}
	return ""
}

// brokenLogEvery — при сломанном конфиге nginx аварийный путь срабатывает на
// КАЖДОМ запросе. Писать каждый раз — затопить лог (так уже было с /metrics
// 10.09.2026), поэтому не чаще раза в этот интервал.
const brokenLogEvery = 10 * time.Minute

var brokenLoggedAt atomic.Int64 // UnixNano последней записи

// logBrokenForwardedFor — сигнал «чините конфиг nginx» с сырыми заголовками.
func logBrokenForwardedFor(r *http.Request, fallback string) {
	now := time.Now().UnixNano()
	last := brokenLoggedAt.Load()
	if last != 0 && now-last < int64(brokenLogEvery) {
		return
	}
	if !brokenLoggedAt.CompareAndSwap(last, now) {
		return // запись уже делает соседний запрос
	}

	attrs := []any{
		"x_forwarded_for", r.Header.Values("X-Forwarded-For"),
		"x_real_ip", r.Header.Get("X-Real-IP"),
		"path", r.URL.Path,
	}
	if fallback != "" {
		slog.Warn("[CLIENT IP] в X-Forwarded-For нет адресов, ключ взят из X-Real-IP — проверьте конфиг nginx",
			append(attrs, "key", fallback)...)
		return
	}
	slog.Error("[CLIENT IP] адрес клиента не определён, вход закрыт — проверьте конфиг nginx", attrs...)
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
// Мусор из заголовка ("192.168.1.82:8090", имя хоста, "unknown", пустая
// строка) не должен становиться частью ключа сессии.
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
