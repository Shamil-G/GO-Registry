// web/export_access.go
//
// Доступ сервисов к JSON-выгрузке входа-выхода (/all_list_off_json).
//
// Вызывает её другой сервис, а не человек в браузере, поэтому сессии SSO у
// вызывающего нет, а SSO здесь и не нужен. Вызывающий представляется адресом:
// список разрешённых лежит в EXPORT_ALLOWED_IPS. Адрес берётся тем же
// mdw.GetClientIP, что и для /metrics, — разбор IP не менялся.
//
// В отличие от /metrics пустой список = закрыто для всех: выгрузка содержит
// данные по сотрудникам.
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

// RequireExportAccess — пускает только адреса из EXPORT_ALLOWED_IPS.
func RequireExportAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := mdw.GetClientIP(r)
		if slices.ContainsFunc(config.Cfg.ExportAllowedIPs, func(a string) bool {
			return strings.TrimSpace(a) == ip
		}) {
			next.ServeHTTP(w, r)
			return
		}

		slog.Info("[Export] доступ закрыт", "ip", ip, "allowed", config.Cfg.ExportAllowedIPs)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "Доступ закрыт.\nВаш адрес: %s\nДобавьте его в EXPORT_ALLOWED_IPS в .env.\n", ip)
	})
}
