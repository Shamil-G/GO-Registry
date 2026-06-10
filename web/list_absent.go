package web

import (
	"bytes"
	"database/sql"
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
		if pageCtx.IsAnonymous {
			slog.Error("Пользователь отсутствует в контексте защищенного маршрута")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var query string
		var queryArgs []any

		// Логика формирования SELECT к Oracle осталась оригинальной и проверенной
		query = `select event_date, time_out, time_in, employee, post, dep_name, cause, head, status, id 
				from register r
				where sysdate between time_out and time_in
				order by event_date desc`
		slog.Info("Оригинальный SELECT выполнен для СУПЕР-АДМИНА", "user", pageCtx.FIO)

		// Выполняем QueryContext к Oracle
		rows, err := storage.DB.QueryContext(r.Context(), query, queryArgs...)
		if err != nil {
			slog.Error("Ошибка получения списка отсутствующих из Oracle", "user", pageCtx.LoginName, "err", err)
			http.Error(w, "Ошибка базы данных: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var list []TimeOffItem
		for rows.Next() {
			var item TimeOffItem
			var rawEventDate, rawTimeOut, rawTimeIn, rawHead sql.NullString

			err := rows.Scan(
				&rawEventDate, &rawTimeOut, &rawTimeIn, &item.Employee,
				&item.Post, &item.DepName, &item.Cause, &rawHead, &item.Status, &item.ID,
			)
			if err != nil {
				slog.Error("Ошибка сканирования строки согласования", "err", err)
				continue
			}

			item.EventDate = rawEventDate.String
			if len(item.EventDate) > 10 {
				item.EventDate = item.EventDate[:10]
			}
			item.TimeOut = rawTimeOut.String
			item.TimeIn = rawTimeIn.String
			item.Head = rawHead.String

			list = append(list, item)
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
