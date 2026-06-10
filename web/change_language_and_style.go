package web

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"gusseynov/GO-Registry/middleware"
	ssoPkg "gusseynov/GO-Registry/sso"
)

// ChangeLangHandler обрабатывает смену языка (GET /language/{lang})
func ChangeLangHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Вытаскиваем язык из параметров URL (чи используется chi.URLParam)
		// Если вы используете chi.NewRouter(), то достаем так:
		lang := chi.URLParam(r, "lang")

		// На случай, если параметр пустой, подстрахуемся
		if lang != "ru" && lang != "kz" {
			lang = "ru"
		}
		// 1. Принудительно выставляем куку темы через бэкенд (для всех!)
		SetCookieLang(w, lang)
		// slog.Debug("[ChangeLangHandler]", "Set Cookie LANG", lang)
		// 2. Проверка авторизации
		userCtx := middleware.GetOrCreatePageCtx(r.Context())
		// slog.Debug("[ChangeLangHandler]", "MIDDLEWARE USER ok", ok, "userTheme", user.Theme)

		// clientIP := middleware.GetIPFromContext(r.Context())
		if !userCtx.IsAnonymous {
			// slog.Info("[ChangeLangHandler]", "SSO. Запрос на смену языка , ip", clientIP, "new_lang", lang)

			err := ssoPkg.Client.Set(r.Context(), userCtx.IP, "lang", lang)
			if err != nil {
				slog.Error("[ChangeLangHandler]", "SSO. Не удалось обновить язык на сервере SSO, ip", userCtx.IP, "err", err)
				// Не ломаем интерфейс пользователю, просто логируем и ведем дальше
			}
		} else {
			slog.Debug("[ChangeLangHandler]", "Anonymous. Язык сохранен локально в куки, ip", userCtx.IP, "lang", lang)
		}
		// 4. Умный редирект обратно на ту страницу, откуда пришел пользователь
		referer := r.Referer()
		if referer == "" {
			referer = "/" // Если пришли по прямой ссылке, кидаем на корень
		}

		http.Redirect(w, r, referer, http.StatusSeeOther)
	}
}

// ChangeStyleAsyncHandler обрабатывает смену темы для всех пользователей (GET /theme/{theme})
func ChangeThemeHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		theme := chi.URLParam(r, "theme")

		if theme != "color" && theme != "dark" {
			theme = "color"
		}

		slog.Debug("[ChangeThemeHandler]", "For Cookie Theme", theme)

		// 1. Принудительно выставляем куку темы через бэкенд (для всех!)
		SetCookieTheme(w, theme)

		// 2. Проверка авторизации
		userCtx := middleware.GetOrCreatePageCtx(r.Context())
		// slog.Debug("[ChangeThemeHandler]", "ok", ok, "USER THEME", user)

		// clientIP := middleware.GetIPFromContext(r.Context())
		if !userCtx.IsAnonymous {
			// Только для авторизованных пушим настройки в GO-SSO
			err := ssoPkg.Client.Set(r.Context(), userCtx.IP, "theme", theme)
			if err != nil {
				slog.Error("[ChangeThemeHandler]", "Не удалось сохранить тему авторизованного юзера в SSO, ip", userCtx.IP, "theme", theme, "err", err)
			}
		} else {
			slog.Debug("[ChangeThemeHandler]", "Смена темы для анонима: сохранено локально в куки, ip", userCtx.IP, "theme", theme)
		}

		// 3. Умный редирект обратно на ту же страницу (как в языках!)
		referer := r.Referer()
		if referer == "" {
			referer = "/"
		}
		// При редиректе контекст не сохраняется
		http.Redirect(w, r, referer, http.StatusSeeOther)
	}
}
