package web

import "net/http"

// Универсальная функция
func SetCookie(w http.ResponseWriter, name, value, path string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:   name,
		Value:  value,
		Path:   path,
		MaxAge: maxAge,
	})
}

// Специализированная функция для темы
func SetCookieTheme(w http.ResponseWriter, theme string) {
	http.SetCookie(w, &http.Cookie{
		Name:   "theme",
		Value:  theme,
		Path:   "/",
		MaxAge: 3 * 24 * 60 * 60,
	})
}

// Специализированная функция для языка
func SetCookieLang(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name:   "lang",
		Value:  lang,
		Path:   "/",
		MaxAge: 3 * 24 * 60 * 60,
	})
}
