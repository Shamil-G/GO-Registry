// middleware/lang.go

package middleware

import (
	"log/slog"
	"net/http"
)

func CookieThemeLang(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := GetOrCreatePageCtx(r.Context())

		if cookie, err := r.Cookie("lang"); err == nil {
			if cookie.Value == "kz" || cookie.Value == "ru" {
				page.Lang = cookie.Value
				slog.Debug("[WithThemeLang]", "GET COOKIE LANG", page.Lang)
			}
		}
		if cookie, err := r.Cookie("theme"); err == nil {
			if cookie.Value == "color" || cookie.Value == "dark" {
				page.Theme = cookie.Value
				slog.Debug("[WithThemeLang]", "GET COOKIE THEME", page.Theme)
			}
		}

		ctx := SavePageCtx(r.Context(), page)
		slog.Debug("[WithThemeLang]", "TEST. ctxPage", page)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
