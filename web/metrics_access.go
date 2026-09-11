// web/metrics_access.go
//
// Ограничение доступа к /metrics (10.09.2026, решение пользователя).
//
// Страница отдаёт дамп счётчиков Prometheus: список всех адресов приложения и
// частоту обращений, имена вызываемых процедур Oracle, состояние пула БД.
// Живёт она вне группы UI-роутов — без PageContext и без авторизации, — то
// есть до этой правки открывалась любому, кто может открыть портал.
//
// Фильтр по адресу, а не basic-auth: Prometheus ходит сюда сам, пароль ему
// пришлось бы куда-то класть, а список из двух-трёх адресов честнее и проще.
//
// ВАЖНО про адрес. Prometheus скребёт портал через nginx (grafana/prometeus.yml:
// target 192.168.1.34:80), поэтому адрес сокета у таких запросов — 127.0.0.1,
// а настоящий адрес приходит в X-Forwarded-For. Поэтому здесь mdw.GetClientIP, а не
// r.RemoteAddr: он берёт заголовок, только если запрос пришёл от доверенного
// прокси (TRUSTED_PROXIES), и потому его нельзя подделать снаружи.
//
// Сравнивается ключ целиком. Если бы Prometheus ходил через корпоративный
// прокси, ключом была бы цепочка "адрес, прокси", а её в METRICS_ALLOWED_IPS
// (список через запятую) не записать — Prometheus должен ходить напрямую.
//
// Пустой METRICS_ALLOWED_IPS = доступ всем, как было. Так сделано намеренно:
// молча оборвать сбор метрик в Grafana — плохой способ узнать о правке. При
// старте про это пишется предупреждение.
package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"gusseynov/GO-Registry/config"
	mdw "gusseynov/GO-Registry/middleware"
)

// RequireMetricsAccess — пускает к /metrics только адреса из METRICS_ALLOWED_IPS.
func RequireMetricsAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed := config.Cfg.MetricsAllowedIPs
		if len(allowed) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		ip := mdw.GetClientIP(r)
		if slices.ContainsFunc(allowed, func(a string) bool {
			return strings.TrimSpace(a) == ip
		}) {
			next.ServeHTTP(w, r)
			return
		}

		logMetricsDenied(ip, allowed)

		// Адрес показывается прямо в ответе, а не только в логе: чтобы добавить
		// себя в список, надо знать, каким тебя видит сервис, — а за nginx это
		// не тот адрес, который человек видит у себя в ipconfig.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Доступ к /metrics закрыт.\nВаш адрес: %s\nДобавьте его в METRICS_ALLOWED_IPS в .env, если он должен быть разрешён.\n", ip)
	})
}

// Отказы в /metrics логируются с подавлением повторов.
//
// Prometheus ходит сюда по расписанию (scrape_interval: 5s в
// grafana/prometeus.yml). Если его адреса нет в списке, он получает отказ
// КАЖДЫЕ ПЯТЬ СЕКУНД — это 17 тысяч одинаковых строк в сутки, и registry.log
// начинает ротироваться быстрее, чем в нём успевает накопиться что-то полезное.
// Именно так и вышло на бою 10.09.2026.
//
// Поэтому с одного адреса пишем не чаще раза в deniedLogEvery, а в строке
// указываем, сколько отказов было подавлено за это время: событие остаётся
// видимым (в том числе если кто-то методично долбится в /metrics), но лог не
// затапливает.
const deniedLogEvery = 10 * time.Minute

// deniedLogCap — потолок на размер карты. Адресов, стучащихся в /metrics,
// должно быть единицы; тысяча означает перебор адресов, и тогда карта просто
// сбрасывается целиком, чтобы не расти в памяти бесконечно.
const deniedLogCap = 1000

type deniedEntry struct {
	last       time.Time
	suppressed int
}

var (
	deniedMu  sync.Mutex
	deniedLog = make(map[string]*deniedEntry)
)

func logMetricsDenied(ip string, allowed []string) {
	deniedMu.Lock()

	if len(deniedLog) > deniedLogCap {
		deniedLog = make(map[string]*deniedEntry)
	}

	e, ok := deniedLog[ip]
	if !ok {
		e = &deniedEntry{}
		deniedLog[ip] = e
	}

	if !e.last.IsZero() && time.Since(e.last) < deniedLogEvery {
		e.suppressed++
		deniedMu.Unlock()
		return
	}

	suppressed := e.suppressed
	e.suppressed = 0
	e.last = time.Now()
	deniedMu.Unlock()

	// Запись вне блокировки: slog может писать в файл, держать под мьютексом
	// ввод-вывод незачем.
	if suppressed > 0 {
		slog.Info("[Metrics] доступ закрыт", "ip", ip, "allowed", allowed,
			"подавлено_повторов", suppressed, "за", deniedLogEvery.String())
		return
	}
	slog.Info("[Metrics] доступ закрыт", "ip", ip, "allowed", allowed)
}
