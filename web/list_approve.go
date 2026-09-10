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
	service "gusseynov/GO-Registry/service"
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
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		// Права, идентификатор и принадлежность заявки — одной проверкой.
		id, ok := approvalTarget(w, r, "refuse")
		if !ok {
			return
		}

		if err := storage.DBExec(r.Context(), "reg.refuse_time_off", id, pageCtx.FIO); err != nil {
			slog.Error("[Approve] ошибка reg.refuse_time_off", "id_reg", id, "boss", pageCtx.FIO, "err", err)
			http.Error(w, i18n.Get(pageCtx.Lang, "ERR_DB"), http.StatusInternalServerError)
			return
		}

		slog.Info("[Approve] заявка отклонена", "id_reg", id, "boss", pageCtx.FIO, "ip", pageCtx.IP)
		http.Redirect(w, r, "/list-to-approve?msg=refused", http.StatusSeeOther)
	}
}

// ApproveTimeOffPost одобряет заявку сотрудника (POST /approve-time-off)
func ApproveTimeOffPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		id, ok := approvalTarget(w, r, "approve")
		if !ok {
			return
		}

		if err := storage.DBExec(r.Context(), "reg.approve_time_off", id, pageCtx.FIO); err != nil {
			slog.Error("[Approve] ошибка reg.approve_time_off", "id_reg", id, "boss", pageCtx.FIO, "err", err)
			http.Error(w, i18n.Get(pageCtx.Lang, "ERR_DB"), http.StatusInternalServerError)
			return
		}

		slog.Info("[Approve] заявка согласована", "id_reg", id, "boss", pageCtx.FIO, "ip", pageCtx.IP)
		http.Redirect(w, r, "/list-to-approve?msg=approved", http.StatusSeeOther)
	}
}

// approvalTarget — проверка «эта заявка вообще из моей епархии».
//
// Одна на согласование и на отказ: расходиться этим двум проверкам нельзя, а
// два скопированных куска расходятся всегда. Возвращает false, если ответ уже
// отправлен клиенту.
//
// Зачем вообще: id заявки лежит скрытым полем формы на /list-to-approve.
// Проверки !pageCtx.IsBoss на входе недостаточно — она отвечает на вопрос
// «руководитель ли ты», а не «твой ли это сотрудник». До этой правки директор
// департамента А, поправив value в инспекторе браузера, согласовывал заявку
// департамента Б. Список при этом фильтровался правильно — но список это
// вёрстка, а не право.
func approvalTarget(w http.ResponseWriter, r *http.Request, action string) (int, bool) {
	pageCtx := middleware.GetOrCreatePageCtx(r.Context())
	lang := pageCtx.Lang

	if !pageCtx.IsBoss {
		slog.Warn("[Approve] отказ: не руководитель", "action", action, "user", pageCtx.LoginName, "post", pageCtx.Post)
		http.Error(w, i18n.Get(lang, "ERR_NOT_BOSS"), http.StatusForbidden)
		return 0, false
	}

	id, err := strconv.Atoi(strings.TrimSpace(r.FormValue("id")))
	if err != nil || id <= 0 {
		slog.Warn("[Approve] некорректный идентификатор заявки",
			"action", action, "id_str", r.FormValue("id"), "user", pageCtx.LoginName)
		http.Error(w, i18n.Get(lang, "ERR_BAD_ID"), http.StatusBadRequest)
		return 0, false
	}

	depName, employee, found, err := service.RegisterDep(r.Context(), id)
	if err != nil {
		slog.Error("[Approve] не удалось прочитать заявку", "action", action, "id_reg", id, "err", err)
		http.Error(w, i18n.Get(lang, "ERR_DB"), http.StatusInternalServerError)
		return 0, false
	}
	if !found {
		// Штатный случай: заявку уже удалили или согласовали с другой вкладки.
		slog.Info("[Approve] заявка не найдена (устаревшая страница)",
			"action", action, "id_reg", id, "user", pageCtx.LoginName)
		http.Error(w, i18n.Get(lang, "ERR_REQUEST_NOT_FOUND"), http.StatusNotFound)
		return 0, false
	}

	if !pageCtx.Scope().Allows(depName) {
		// Warn: событие аудита. В списке этой заявки не было, значит форму
		// правили руками.
		slog.Warn("[Approve] попытка решить заявку чужого департамента",
			"action", action, "id_reg", id, "request_dep", depName, "employee", employee,
			"user", pageCtx.LoginName, "user_dep", pageCtx.DepName,
			"scope", pageCtx.Scope().Departments, "ip", pageCtx.IP)
		http.Error(w, i18n.Get(lang, "ERR_OTHER_DEPARTMENT"), http.StatusForbidden)
		return 0, false
	}

	return id, true
}
