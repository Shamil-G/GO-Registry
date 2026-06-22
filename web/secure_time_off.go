package web

import (
	"bytes"
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"gusseynov/GO-Registry/middleware"
	message "gusseynov/GO-Registry/service"
	"gusseynov/GO-Registry/service/i18n"
	"gusseynov/GO-Registry/storage"

	go_ora "github.com/sijms/go-ora/v2"
)

const query string = `select 
					to_char(event_date, 'DD.MM.YYYY') as event_date, 
					to_char(time_out, 'DD.MM.YYYY HH24:MI') as time_out, 
					to_char(time_in, 'DD.MM.YYYY HH24:MI') as time_in, 
					employee, 
					post, 
					dep_name, 
					coalesce(cause, ' ') as cause, 
					coalesce(head, ' ') as head, 
					status, 
					' ' as time_fact,			-- заглушка для поля TimeFact
					trunc(sysdate - trunc(time_out, 'MM')) as cnt_days, -- расчет для поля CntDays
					status as status2, 									-- дубликат для поля Status2
					id
				from register r
				where trunc(event_date, 'MM') = trunc(sysdate, 'MM')
				and status = 3
				order by event_date desc`

// ListToApproveGet отображает список активных заявок на утверждение (GET /list-to-approve)
// TimeOffGet отображает список активных заявок сотрудника (GET /time-off)
func SecureTimeOffGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Извлекаем готовый пакет данных из нашей объединенной мидлвари Authorize
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		// 2. ВЫЗЫВАЕМ SELECT ЗАПРОС К ORACLE ЧЕРЕЗ SQLX (Без login_name)

		var list []TimeOffItem
		// Передаем только контекст и запрос. ssoUser.LoginName убран, так как в SQL нет плейсхолдеров.

		err := storage.DBSelectMany(r.Context(), "secure_time_off", &list, query)
		if err != nil {
			http.Error(w, "Ошибка базы данных: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Постобработка: аккуратно подрезаем длины строк дат прямо в полученном слайсе
		for i := range list {
			if len(list[i].EventDate) > 10 {
				list[i].EventDate = list[i].EventDate[:10]
			}
			if len(list[i].TimeOut) > 16 {
				list[i].TimeOut = list[i].TimeOut[:16]
			}
			if len(list[i].TimeIn) > 16 {
				list[i].TimeIn = list[i].TimeIn[:16]
			}
		}

		// Читаем сообщения для базового шаблона
		messages := message.GetAllMessage(r.Context())

		msgParam := r.URL.Query().Get("msg")
		messageText := ""
		if msgParam == "success" {
			messageText = i18n.Get(pageCtx.Lang, "TIME_OFF_SAVED")
			if messageText == "TIME_OFF_SAVED" {
				messageText = "Запись успешно сохранена!"
			}
		}

		// 3. Формируем структуру данных
		data := ViewTimeOff{
			BasePageContext: pageCtx,
			Message:         messageText,
			ListTimeOff:     list,
			AllMessages:     messages,
		}

		// 4. Компиляция шаблонов с привязкой i18n
		tmpl, err := template.New("base.html").Funcs(template.FuncMap{
			"res_value": func(key string) string {
				return i18n.Get(pageCtx.Lang, key)
			},
		}).ParseFiles(
			"templates/base.html",
			"templates/secure_time_off.html",
			"templates/sidebar.html", // Вынесенная инфо-панель
		)

		if err != nil {
			slog.Error("Ошибка компиляции шаблона secure_time_off", "err", err)
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

// TimeOffPost обрабатывает отправку формы и регистрирует новое отсутствие (POST /time-off)
// SecureTimeOffPost обрабатывает отправку формы и регистрирует новое отсутствие (POST /secure-time-off)
// SecureTimeOffPost обрабатывает отправку формы и регистрирует новое отсутствие (POST /secure-time-off)
func SecureTimeOffPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Извлекаем готовый пакет данных из нашей мидлвари Authorize
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())
		if pageCtx.IsAnonymous {
			slog.Error("Пользователь отсутствует в контексте при отправке формы secure-time-off")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Читаем данные из формы
		dateOut := r.FormValue("date_out")
		dateIn := r.FormValue("date_in")
		employee := r.FormValue("employee")
		depName := r.FormValue("dep_name")
		post := r.FormValue("post")
		cause := r.FormValue("cause")

		// Валидация входных данных
		if dateOut == "" || dateIn == "" || len(cause) < 12 {
			http.Error(w, "Заполните все поля. Длина причины должна быть не менее 12 символов.", http.StatusBadRequest)
			return
		}

		// Форматируем даты под маску YYYY-MM-DD HH24:MI
		formattedDateOut := strings.Replace(dateOut, "T", " ", 1)
		formattedDateIn := strings.Replace(dateIn, "T", " ", 1)

		// Извлекаем сохраненный в мидлвари IP-адрес для системных логов
		clientIP := middleware.GetIPFromContext(r.Context())
		slog.Info("Вызов PL/SQL функции reg.add_secure_reg через анонимный блок", "user", pageCtx.LoginName, "ip", clientIP)

		// Вызов функции через именованные параметры для предотвращения ошибок UDT
		query := `BEGIN :ret_val := reg.add_secure_reg(:d_out, :d_in, :emp, :pst, :dep, :caus, :fio); END;`

		var oracleResult string

		// Явно выделяем строковые параметры для стабильности биндинга в go_ora
		var (
			dOutParam string = formattedDateOut
			dInParam  string = formattedDateIn
			empParam  string = employee
			pstParam  string = post
			depParam  string = depName
			causParam string = cause
			fioParam  string = pageCtx.FIO
		)

		// Используем ExecContext напрямую со стандартным *sql.DB.
		// _, err := storage.DB.DB.ExecContext(r.Context(), query,
		err := storage.DBExecNamed(r.Context(), query,
			"reg.add_secure_reg",
			sql.Named("ret_val", go_ora.Out{Dest: &oracleResult, Size: 256}),
			sql.Named("d_out", dOutParam),
			sql.Named("d_in", dInParam),
			sql.Named("emp", empParam),
			sql.Named("pst", pstParam),
			sql.Named("dep", depParam),
			sql.Named("caus", causParam),
			sql.Named("fio", fioParam),
		)

		// Обработка системных ошибок связи с Oracle
		if err != nil {
			http.Error(w, "Ошибка связи с базой данных: "+err.Error(), http.StatusInternalServerError)
			return
		}

		slog.Debug("Результат выполнения функции reg.add_secure_time_off", "status", oracleResult)

		// АНАЛИЗИРУЕМ ТЕКСТОВЫЙ ОТВЕТ ИЗ ORACLE
		if oracleResult != "Success" {
			slog.Error("TIME_OFF Блок отклонен", "USER", pageCtx.LoginName, "REJECT_REASON", oracleResult)

			// 🚀 ВМЕСТО ЧЕРНОГО ОКНА — КРАСИВО РЕНДЕРИМ ФОРМУ ОБРАТНО С ОШИБКОЙ

			// Дозапрашиваем актуальный список записей (точно такой же запрос, как в отлаженном GET)
			var list []TimeOffItem

			_ = storage.DBSelectMany(r.Context(), "secure_time_off_post", &list, query)

			// Получаем сообщения для базового шаблона
			messages := message.GetAllMessage(r.Context())

			// Собираем data. Ошибку Oracle кладем напрямую в Message
			data := ViewTimeOff{
				BasePageContext: pageCtx,
				Message:         oracleResult, // Текст ошибки уйдет на форму в {{ .Message }}
				ListTimeOff:     list,         // Сохраняем имя для шаблона secure_time_off
				AllMessages:     messages,
			}

			// Компилируем и возвращаем шаблон
			tmpl, err := template.New("base.html").Funcs(template.FuncMap{
				"res_value": func(key string) string { return i18n.Get(pageCtx.Lang, key) },
			}).ParseFiles(
				"templates/base.html",
				"templates/secure_time_off.html",
			)

			if err != nil {
				slog.Error("Ошибка компиляции шаблона при возврате ошибки POST", "err", err)
				http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
				return
			}

			var buf bytes.Buffer
			_ = tmpl.ExecuteTemplate(&buf, "base.html", data)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = buf.WriteTo(w)
			return // Важно: прерываем выполнение, чтобы не сработал редирект ниже
		}

		slog.Debug("SECURE_TIME_OFF Успешно добавлено", "USER", pageCtx.LoginName, "STATUS", oracleResult)

		// Редирект обратно в журнал отсутствий с параметром успеха
		http.Redirect(w, r, "/secure-time-off?msg=success", http.StatusSeeOther)
	}
}
