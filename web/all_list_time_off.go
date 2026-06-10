package web

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"gusseynov/GO-Registry/middleware"
	message "gusseynov/GO-Registry/service"
	"gusseynov/GO-Registry/service/i18n"
	"gusseynov/GO-Registry/storage"
)

type AllTimeOffPageData struct {
	*middleware.BasePageContext                       // Встраиваем базовый контекст напрямую
	ListTimeOff                 []TimeOffItem         // Массив записей для таблицы
	FltMonth                    string                // Выбранный месяц для инпута (например, "2026-06")
	AllMessages                 []message.MessageItem // <-- ДОБАВЛЯЕМ ЭТО ПОЛЕ ДЛЯ base.html
}

// AllListTimeOff отображает общий список отсутствий с фильтрацией по месяцам (GET/POST /all-time-off)
// AllListTimeOff отображает общий список отсутствий с фильтрацией по месяцам (GET/POST /all-time-off)
func AllListTimeOff() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Извлекаем готовый пакет данных из мидлвари
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())
		if pageCtx.IsAnonymous {
			slog.Error("Пользователь отсутствует в контексте защищенного маршрута общего списка")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// 2. Определяем выбранный месяц (GET - текущий, POST - из формы)
		selectedMonth := time.Now().Format("2006-01")
		if r.Method == http.MethodPost {
			formMonth := r.FormValue("flt_month")
			if formMonth != "" {
				selectedMonth = formMonth
			}
		}

		// 3. Вызываем SELECT запрос к Oracle
		query := `SELECT 
					TO_CHAR(event_date, 'DD.MM.YYYY') AS event_date, 
					TO_CHAR(time_out, 'DD.MM.YYYY HH24:MI') AS time_out, 
					TO_CHAR(time_in, 'DD.MM.YYYY HH24:MI') AS time_in, 
					employee, post, dep_name, 
					COALESCE(cause, ' ') AS cause, 
					COALESCE(head, ' ') AS head, 
					status, 
					COALESCE(TO_CHAR(time_fact, 'DD.MM.YYYY HH24:MI'), ' ') AS time_fact,
					TO_NUMBER(TRUNC(sysdate) - TRUNC(time_out)) AS cnt_days, 
					status AS status2, 
					id
				  FROM register r
				  WHERE TRUNC(event_date, 'MM') = TO_DATE(:1, 'YYYY-MM')
				  ORDER BY r.event_date DESC`

		var list []TimeOffItem
		err := storage.DB.SelectContext(r.Context(), &list, query, selectedMonth)
		if err != nil {
			slog.Error("Ошибка получения общего списка отсутствий через sqlx", "month", selectedMonth, "err", err)
			http.Error(w, "Ошибка базы данных: "+err.Error(), http.StatusInternalServerError)
			return
		}
		messages := message.GetAllMessage(r.Context())
		// 4. ИНИЦИАЛИЗИРУЕМ НАШ НОВЫЙ ВЫДЕЛЕННЫЙ ОБЪЕКТ
		data := AllTimeOffPageData{
			BasePageContext: pageCtx,
			ListTimeOff:     list,
			FltMonth:        selectedMonth,
			AllMessages:     messages,
		}

		// 5. Компиляция шаблонов с привязкой i18n
		tmpl, err := template.New("base.html").Funcs(template.FuncMap{
			"res_value": func(key string) string {
				return i18n.Get(pageCtx.Lang, key)
			},
		}).ParseFiles(
			"templates/base.html",
			"templates/all_list_time_off.html",
		)

		if err != nil {
			slog.Error("Ошибка компиляции общего шаблона отсутствий", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Безопасный рендеринг через буфер
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
			slog.Error("Ошибка выполнения общего шаблона отсутствий", "err", err)
			http.Error(w, "Ошибка рендеринга", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = buf.WriteTo(w)
	}
}
