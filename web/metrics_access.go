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
// а настоящий адрес приходит в X-Real-IP. Поэтому здесь mdw.GetClientIP, а не
// r.RemoteAddr: он берёт заголовок, только если запрос пришёл от доверенного
// прокси (TRUSTED_PROXIES), и потому его нельзя подделать снаружи.
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

		slog.Info("[Metrics] доступ закрыт", "ip", ip, "allowed", allowed)

		// Адрес показывается прямо в ответе, а не только в логе: чтобы добавить
		// себя в список, надо знать, каким тебя видит сервис, — а за nginx это
		// не тот адрес, который человек видит у себя в ipconfig.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Доступ к /metrics закрыт.\nВаш адрес: %s\nДобавьте его в METRICS_ALLOWED_IPS в .env, если он должен быть разрешён.\n", ip)
	})
}
