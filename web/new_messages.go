package web

import (
	// "html/template"
	"bytes"
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"

	"gusseynov/GO-Registry/middleware"
	"gusseynov/GO-Registry/service/i18n"
	"gusseynov/GO-Registry/storage"
)

type ViewMessage struct {
	// 💡 Встраиваем базовый контекст (подтянет UserName, DepName, Lang, Theme)
	*middleware.BasePageContext
	Messages string
	// Если в шаблоне list_approve.html используются старые названия,
	// мы можем временно продублировать их для обратной совместимости:
	// UserName string
	// UserPost string
	// UserDep  string
}

func NewMessageGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Извлекаем уже готовый и посчитанный контекст из нашей мидлвари Authorize
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		data := ViewTimeOff{
			BasePageContext: pageCtx,
			Message:         "",
		}
		// 4. Компиляция шаблонов с привязкой i18n
		tmpl, err := template.New("base.html").Funcs(template.FuncMap{
			"res_value": func(key string) string {
				return i18n.Get(pageCtx.Lang, key)
			},
		}).ParseFiles(
			"templates/base.html",
			"templates/new_message.html",
		)

		if err != nil {
			slog.Error("Ошибка компиляции шаблона new_message", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Безопасный рендеринг через буфер
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
			slog.Error("Ошибка выполнения шаблона модуля отсутствий", "err", err)
			http.Error(w, "Ошибка рендеринга", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = buf.WriteTo(w)

	}
}

// AllListReportHandler генерирует Excel отчет и отправляет его пользователю на скачивание
func NewMessagePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		newMessage := r.FormValue("new_message")
		query := `begin reg.new_message(:employee, :dep_name, :message); end;`

		// Используем ExecContext напрямую со стандартным *sql.DB.
		_, err := storage.DB.DB.ExecContext(r.Context(), query,
			sql.Named("employee", pageCtx.FIO),
			sql.Named("dep_name", pageCtx.DepName),
			sql.Named("message", newMessage),
		)

		// Обработка системных ошибок связи с Oracle
		if err != nil {
			slog.Error("Критическая ошибка выполнения анонимного блока reg.add_secure_time_off", "err", err)
			http.Error(w, "Ошибка связи с базой данных: "+err.Error(), http.StatusInternalServerError)
			return
		}

		slog.Debug("Функция reg.new_message выполнена")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
