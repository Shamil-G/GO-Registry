package web

import (
	"bytes"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"gusseynov/GO-Registry/config"
	"gusseynov/GO-Registry/middleware"
	"gusseynov/GO-Registry/service/i18n"
	"gusseynov/GO-Registry/storage"
)

// ViewApprove — структура данных для рендеринга шаблона list_to_approve.html
type ViewApprove struct {
	*middleware.BasePageContext // 💡 Встраиваем базовый контекст (подтянет UserName, DepName, Lang, Theme)
	ListToApprove               []TimeOffItem
	AllMessages                 []MessageItem
}

// ListToApproveGet отображает список активных заявок на утверждение (GET /list-to-approve)
// ListToApproveGet отображает список активных заявок на утверждение (GET /list-to-approve)
func ListToApproveGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Достаем готовый пакет данных из нашей объединенной мидлвари
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		// 2. Навешиваем щит безопасности, используя готовый флаг из мидлвари
		if !pageCtx.IsBoss {
			slog.Warn("Отказ в доступе к панели согласования", "user", pageCtx.LoginName, "post", pageCtx.Post)
			http.Error(w, "Доступ запрещен. Вы не являетесь руководителем подразделения.", http.StatusForbidden)
			return
		}

		// Повторно вычисляем статус админа для разделения SQL логики
		isAdmin := config.IsSuperAdmin(pageCtx.FIO)
		isBigBoss := len(pageCtx.SubordinateOU) > 0
		isSmallBoss := config.IsBossPost(pageCtx.Post)

		var query string
		var queryArgs []any

		var err error
		// Выполняем QueryContext к Oracle
		var list []TimeOffItem
		// Логика формирования SELECT к Oracle осталась оригинальной и проверенной
		if isAdmin {
			query = `SELECT 
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
					 FROM register r
					 WHERE trunc(event_date,'MM') >= trunc(sysdate,'MM')-5
					 AND   status = 0
					 ORDER BY event_date DESC`
			slog.Info("Выполнен запрос LIST_APPROVE для СУПЕР-АДМИНА", "user", pageCtx.LoginName)
			err = storage.DBSelectMany(r.Context(), "list_approve", &list, query)
		} else {
			var targetDepartments []string
			if isBigBoss {
				targetDepartments = append(targetDepartments, pageCtx.SubordinateOU...)
			} else if isSmallBoss && pageCtx.DepName != "" {
				targetDepartments = append(targetDepartments, pageCtx.DepName)
			}

			placeholders := make([]string, len(targetDepartments))
			queryArgs = make([]any, len(targetDepartments))
			for i, dep := range targetDepartments {
				placeholders[i] = fmt.Sprintf(":%d", i+1)
				queryArgs[i] = dep
			}
			inClause := strings.Join(placeholders, ", ")

			query = fmt.Sprintf(`SELECT 
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
					  FROM register r
					  WHERE trunc(event_date,'MM') >= trunc(sysdate,'MM')-5
					  AND   status = 0
					  AND   dep_name IN (%s)
					  ORDER BY event_date DESC`, inClause)

			err = storage.DBSelectMany(r.Context(), "list_approve", &list, query, queryArgs...)
			slog.Info("Оригинальный SELECT выполнен для РУКОВОДИТЕЛЯ", "user", pageCtx.LoginName, "кол-во департаментов", len(targetDepartments))
		}

		if err != nil {
			http.Error(w, "Ошибка базы данных: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 3. Собираем контекст страницы. Основные поля наследуются автоматически!
		data := ViewApprove{
			BasePageContext: pageCtx, // Передаем весь пакет из мидлвари одним махом
			ListToApprove:   list,
			AllMessages:     []MessageItem{},
		}

		// 4. Компиляция шаблонов с привязкой i18n на основе вычисленного в мидлвари языка
		tmpl, err := template.New("base.html").Funcs(template.FuncMap{
			"res_value": func(key string) string {
				return i18n.Get(pageCtx.Lang, key)
			},
		}).ParseFiles(
			"templates/base.html",
			"templates/list_approve.html",
			"templates/sidebar.html", // Вынесенная инфо-панель
		)

		if err != nil {
			slog.Error("Ошибка компиляции шаблона list_to_approve", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Безопасный рендеринг через буфер
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
			slog.Error("Ошибка выполнения шаблона панели босса", "err", err)
			http.Error(w, "Ошибка рендеринга", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = buf.WriteTo(w)
	}
}

// RefuseTimeOffPost отклоняет заявку сотрудника (POST /refuse-time-off)
func RefuseTimeOffPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Контроль авторизации и прав руководителя через единый контекст
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		// Дополнительный щит: если это не босс и не админ, рубим запрос сразу
		if !pageCtx.IsBoss {
			slog.Error("Отказ в доступе к процедуре отклонения", "user", pageCtx.LoginName)
			http.Error(w, "Доступ запрещен", http.StatusForbidden)
			return
		}
		// 2. Считываем ID из POST-параметра формы
		idStr := r.FormValue("id")
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			slog.Warn("Некорректный ID в POST-запросе отказа", "id_str", idStr)
			http.Error(w, "Неверный идентификатор записи", http.StatusBadRequest)
			return
		}

		slog.Debug("Запуск процедуры reg.refuse_time_off", "id_reg", id, "boss", pageCtx.FIO, "ip", pageCtx.IP)

		// 3. Вызываем оригинальную хранимую процедуру отказа в Oracle
		// query := `BEGIN reg.refuse_time_off(:1, :2); END;`

		// // _, err = storage.DB.ExecContext(r.Context(), query, id, pageCtx.FIO)
		err = storage.DBExec(r.Context(), "reg.refuse_time_off", id, pageCtx.FIO)
		if err != nil {
			http.Error(w, "Ошибка базы данных при отклонении: "+err.Error(), http.StatusInternalServerError)
			return
		}

		slog.Debug("Заявка отклонена", "id_reg", id, "boss", pageCtx.FIO)

		// 4. Редирект обратно в панель с параметром отказа
		http.Redirect(w, r, "/list-to-approve?msg=refused", http.StatusSeeOther)
	}
}

// ApproveTimeOffPost одобряет заявку сотрудника (POST /approve-time-off)
func ApproveTimeOffPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Контроль авторизации и прав руководителя через единый контекст мидлвари Authorize
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		// Дополнительный щит безопасности: если это не босс и не админ, рубим запрос сразу
		if !pageCtx.IsBoss {
			slog.Error("Отказ в доступе к процедуре одобрения", "user", pageCtx.LoginName)
			http.Error(w, "Доступ запрещен. Вы не являетесь руководителем.", http.StatusForbidden)
			return
		}

		// 2. Считываем ID из POST-параметра формы
		idStr := r.FormValue("id")
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			slog.Warn("Некорректный ID в POST-запросе согласования", "id_str", idStr)
			http.Error(w, "Неверный идентификатор записи", http.StatusBadRequest)
			return
		}

		clientIP := middleware.GetIPFromContext(r.Context())
		slog.Debug("Запуск процедуры reg.approve_time_off", "id_reg", id, "boss", pageCtx.FIO, "ip", clientIP)

		// 3. Вызываем оригинальную хранимую процедуру пакета Oracle
		// query := `BEGIN reg.approve_time_off(:1, :2); END;`

		// Передаем ID (:1) и Полное ФИО босса (:2) строго по контракту
		// _, err = storage.DB.ExecContext(r.Context(), query, id, pageCtx.FIO)
		err = storage.DBExec(r.Context(), "reg.approve_time_off", id, pageCtx.FIO)
		if err != nil {
			http.Error(w, "Ошибка базы данных при согласовании: "+err.Error(), http.StatusInternalServerError)
			return
		}

		slog.Debug("Заявка успешно одобрена", "id_reg", id, "boss", pageCtx.FIO)

		// 4. Редирект обратно в панель с параметром успеха
		http.Redirect(w, r, "/list-to-approve?msg=approved", http.StatusSeeOther)
	}
}
