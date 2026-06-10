// middleware/auth.go

package middleware

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"gusseynov/GO-Registry/config" // Конфиг для проверки ролей боссов
	"gusseynov/GO-Registry/sso"    // Наш пакет SSO
)

// Authorize — единая middleware: извлекает IP, проверяет сессию в SSO и собирает контекст страниц
func Authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// 1. Проверяем исключения для статики и публичных страниц
		if strings.HasPrefix(path, "/static/") || path == "/bd" {
			slog.Debug("[AUTH]", "Пропуск публичного пути без проверки SSO, path", path)
			next.ServeHTTP(w, r)
			return
		}

		page := GetOrCreatePageCtx(r.Context())
		slog.Debug("[AUTH]", "1. Проверка сессии, theme", page.Theme, "lang", page.Lang)

		// 2. Извлекаем IP-адрес клиента оригинальным проверенным методом
		page.IP = GetClientIP(r)

		slog.Debug("[AUTH]", "2. Проверка сессии, path", path, "ip", page.IP)

		// 3. Вызываем метод нашего глобального sso.Client
		ssoUser, err := sso.Client.CheckSession(r.Context(), page.IP)
		if err != nil {
			page.IsAnonymous = true

			ctx := SavePageCtx(r.Context(), page)
			slog.Debug("[AUTH]", "ANONYN USER, UserKey", UserKey, "page", page)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		slog.Debug("[AUTH]", "Сессия валидна, user", ssoUser.LoginName, "path", path, "theme", ssoUser.Theme, "LANG", ssoUser.Lang)

		// 5. Расчет иерархии ролей руководителя по правилам пакета config
		isAdmin := config.IsSuperAdmin(ssoUser.FIO)
		isBigBoss := len(ssoUser.SubordinateOU) > 0
		isSmallBoss := config.IsBossPost(ssoUser.Post)
		isBoss := isAdmin || isBigBoss || isSmallBoss

		// 6. Формируем единый базовый контекст для UI шаблонов
		page.FIO = ssoUser.FIO
		page.LoginName = ssoUser.FIO
		page.Post = ssoUser.Post
		page.DepName = ssoUser.DepName
		page.IsAnonymous = false
		page.IsBoss = isBoss
		page.LegacyName = ssoUser.LegacyName
		page.LoginName = ssoUser.LoginName
		page.RfbnID = ssoUser.RfbnID
		page.SubordinateOU = ssoUser.SubordinateOU

		// 7. Сохраняем и контекст страницы, и сам IP в context запроса (на случай, если IP нужен в логике)
		ctx := SavePageCtx(r.Context(), page)

		slog.Debug("[AUTH]", "REAL USER, UserKey", UserKey, "clientIP", page.IP, "NOW THEME", page.Theme, "NOW LANG", page.Lang)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetIPFromContext(ctx context.Context) string {
	page := GetOrCreatePageCtx(ctx)
	return page.IP
}

// GetClientIP — оригинальная функция извлечения IP из HTTP заголовков
func GetClientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}

	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		if host == "::1" {
			return "127.0.0.1"
		}
		return host
	}

	return r.RemoteAddr
}
