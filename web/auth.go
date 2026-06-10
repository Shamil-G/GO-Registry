package web

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"

	"gusseynov/GO-Registry/middleware"
	"gusseynov/GO-Registry/service/i18n"
	ssoPkg "gusseynov/GO-Registry/sso"
)

// renderLoginForm — единая функция рендеринга страницы входа
func renderLoginForm(w http.ResponseWriter, r *http.Request, errorMsg string) {
	// 1. Динамически определяем язык анонима (сначала кука, потом контекст)
	// lang := "ru"
	pageCtx := middleware.GetOrCreatePageCtx(r.Context())

	slog.Info("[LOGIN_FORM]", "LANG", pageCtx.Lang)
	slog.Info("[LOGIN_FORM]", "THEME", pageCtx.Theme)

	tmpl, err := template.New("base.html").Funcs(template.FuncMap{
		"res_value": func(key string) string {
			return i18n.Get(pageCtx.Lang, key) // Теперь локализация динамическая!
		},
	}).ParseFiles(
		"templates/base.html",
		"templates/login.html",
	)

	if err != nil {
		slog.Error("Ошибка компиляции шаблона логина", "err", err)
		http.Error(w, "Внутренняя ошибка сервера", http.StatusInternalServerError)
		return
	}

	// Передаем данные в шаблон. errorMsg попадет в {{ if .Error }}
	data := map[string]any{
		"Theme":       pageCtx.Theme,
		"Lang":        pageCtx.Lang,
		"IsAnonymous": true,
		"Error":       errorMsg,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		slog.Error("Ошибка выполнения шаблона логина", "err", err)
		http.Error(w, "Ошибка рендеринга", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = buf.WriteTo(w)
}

// LoginFormGet отображает страницу авторизации (GET /login)
func LoginFormGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Отображаем пустую форму без ошибок
		renderLoginForm(w, r, "")
	}
}

// LoginPost обрабатывает отправку формы (POST /login)
func LoginPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		password := r.FormValue("password")

		clientIP := middleware.GetClientIP(r)
		slog.Info("Попытка авторизации", "user", username, "ip", clientIP)

		// Вызываем метод sso.Client
		ssoUser, err := ssoPkg.Client.Login(r.Context(), username, password, clientIP)
		if err != nil {
			slog.Warn("SSO отклонил авторизацию", "user", username, "ip", clientIP, "err", err)

			// 1. Получаем текущий язык из контекста (чтобы ошибка была на нужном языке)
			lang, ok := r.Context().Value("lang").(string)
			if !ok || lang == "" {
				lang = "ru"
			}

			// 2. Достаем переведенный текст ошибки по ключу "AUTH_FAILED"
			translatedError := i18n.Get(lang, "AUTH_FAILED")

			// 3. Рендерим форму обратно с правильным переводом
			renderLoginForm(w, r, translatedError)
			return
		}

		slog.Info("Успешная авторизация в SSO", "user", username, "ip", clientIP)
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())
		slog.Debug("[LOGIN_POST]", "GET COOKIE THEME & LANG. cookie_theme", pageCtx.Theme, "cookie_lang", pageCtx.Lang)

		// 1. Инициализируем значения дефолтами из SSO
		userLang := ssoUser.Lang
		slog.Info("[LOGIN_POST]", "GET userLang", userLang)
		if userLang == "" {
			userLang = pageCtx.Lang
			if err := ssoPkg.Client.Set(r.Context(), clientIP, "lang", userLang); err != nil {
				slog.Warn("Не удалось сохранить язык в SSO при логине", "err", err)
			}
		}
		userTheme := ssoUser.Theme
		if userTheme == "" {
			userTheme = pageCtx.Theme
			if err := ssoPkg.Client.Set(r.Context(), clientIP, "theme", userTheme); err != nil {
				slog.Warn("Не удалось сохранить тему в SSO при логине", "err", err)
			}
		}
		slog.Info("[LOGIN_POST]", "GET userTheme", userTheme)
		if userLang != pageCtx.Lang {
			SetCookieLang(w, userLang)
		}
		if userTheme != pageCtx.Theme {
			SetCookieTheme(w, userTheme)
		}

		// !!! КОНЕЦ ИНТЕГРАЦИИ !!!
		// Редирект на корень приложения
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// LogoutGet обрабатывает выход (GET /logout)
func LogoutGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientIP := middleware.GetIPFromContext(r.Context())
		slog.Info("Запрос на закрытие сессии", "ip", clientIP)

		if err := ssoPkg.Client.CloseSession(r.Context(), clientIP); err != nil {
			slog.Error("Ошибка закрытия сессии в SSO", "ip", clientIP, "err", err)
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
