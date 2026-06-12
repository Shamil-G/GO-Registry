package web

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"

	"gusseynov/GO-Registry/middleware"
	"gusseynov/GO-Registry/service/i18n"
	"gusseynov/GO-Registry/storage"
)

// ViewApprove — структура данных для рендеринга шаблона list_to_approve.html
type ViewAbsent struct {
	*middleware.BasePageContext // 💡 Встраиваем базовый контекст (подтянет UserName, DepName, Lang, Theme)
	ListAbsent                  []TimeOffItem
	AllMessages                 []MessageItem
	// Если в шаблоне list_approve.html используются старые названия,
	// мы можем временно продублировать их для обратной совместимости:
	Style    string
	Lang     string
	UserName string
	UserPost string
	UserDep  string
}

// ListToApproveGet отображает список активных заявок на утверждение (GET /list-to-approve)
// ListToApproveGet отображает список активных заявок на утверждение (GET /list-to-approve)
func ListAbsent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Достаем готовый пакет данных из нашей объединенной мидлвари
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		var query string
		// Логика формирования SELECT к Oracle осталась оригинальной и проверенной
		query = `select 
					to_char(event_date, 'DD.MM.YYYY') as event_date, 
					TO_CHAR(time_out, 'YYYY-MM-DD HH24:MI') as time_out, 
					TO_CHAR(time_in, 'YYYY-MM-DD HH24:MI') as time_in, 
					employee, 
					post, 
					coalesce(dep_name, ' ') as dep_name, 
					coalesce(cause, ' ') as cause, 
					coalesce(head, ' ') as head, 
					status, 
					id 
				from register r
				where sysdate between time_out and time_in
				order by event_date desc`
		slog.Debug("Оригинальный SELECT выполнен для СУПЕР-АДМИНА", "user", pageCtx.FIO)

		// Выполняем QueryContext к Oracle
		var list []TimeOffItem
		err := storage.DBSelectMany(r.Context(), "list_absent", &list, query)
		if err != nil {
			http.Error(w, "Ошибка базы данных: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 3. Собираем контекст страницы. Основные поля наследуются автоматически!
		data := ViewAbsent{
			BasePageContext: pageCtx, // Передаем весь пакет из мидлвари одним махом
			ListAbsent:      list,
			AllMessages:     []MessageItem{},
		}

		// 4. Компиляция шаблонов с привязкой i18n на основе вычисленного в мидлвари языка
		tmpl, err := template.New("base.html").Funcs(template.FuncMap{
			"res_value": func(key string) string {
				return i18n.Get(pageCtx.Lang, key)
			},
		}).ParseFiles(
			"templates/base.html",
			"templates/list_absent.html",
		)

		if err != nil {
			slog.Error("Ошибка компиляции шаблона list_absent", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Безопасный рендеринг через буфер
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
			slog.Error("Ошибка выполнения шаблона list_absent", "err", err)
			http.Error(w, "Ошибка рендеринга", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = buf.WriteTo(w)
	}
}
