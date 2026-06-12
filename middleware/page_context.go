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
func PageContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// 1. Проверяем исключения для статики и публичных страниц
		if strings.HasPrefix(path, "/static/") {
			slog.Debug("[PCTX]", "Пропуск публичного пути без проверки SSO, path", path)
			next.ServeHTTP(w, r)
			return
		}
		page := GetOrCreatePageCtx(r.Context())
		// 2. Извлекаем IP-адрес клиента оригинальным проверенным методом
		page.IP = GetClientIP(r)
		// Даже для Анонима надо выставить Theme & Lang
		if cookie, err := r.Cookie("lang"); err == nil {
			if cookie.Value == "kz" || cookie.Value == "ru" {
				page.Lang = cookie.Value
			}
		}
		if cookie, err := r.Cookie("theme"); err == nil {
			if cookie.Value == "color" || cookie.Value == "dark" {
				page.Theme = cookie.Value
			}
		}

		// slog.Debug("[PCTX]", "theme", page.Theme, "lang", page.Lang, "ip", page.IP, "path", path)

		// 3. Вызываем метод нашего глобального sso.Client
		ssoUser, err := sso.Client.CheckSession(r.Context(), page.IP)
		if err != nil {
			page.IsAnonymous = true

			ctx := SavePageCtx(r.Context(), page)

			if config.IsPublicPath(path) {
				slog.Debug("[PCTX] Публичный путь, пропускаем без SSO", "path", path)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			slog.Error("[PCTX]", "Попытка анонимного входа на ", path, "c адреса", page.IP)
			http.Redirect(w, r, config.Cfg.LOGIN_PAGE, http.StatusSeeOther)
			return
		}

		page.IsAnonymous = false

		// 5. Расчет иерархии ролей руководителя по правилам пакета config
		isAdmin := config.IsSuperAdmin(ssoUser.FIO)
		isBigBoss := len(ssoUser.SubordinateOU) > 0
		isSmallBoss := config.IsBossPost(ssoUser.Post)
		isBoss := isAdmin || isBigBoss || isSmallBoss

		// 6. Формируем единый базовый контекст для UI шаблонов
		page.FIO = ssoUser.FIO
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

		slog.Info("[PCTX]", "REAL USER. LoginName", page.LoginName, "IP", page.IP, "FIO", page.FIO,
			"THEME", page.Theme, "LANG", page.Lang, "IsBoss", page.IsBoss, "IsAdmin", isAdmin, "Post", page.Post,
			"SUBORDINATE", page.SubordinateOU)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetIPFromContext(ctx context.Context) string {
	return GetOrCreatePageCtx(ctx).IP
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
