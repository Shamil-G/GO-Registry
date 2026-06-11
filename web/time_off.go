package web

import (
	"bytes"
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"gusseynov/GO-Registry/middleware"
	message "gusseynov/GO-Registry/service"
	"gusseynov/GO-Registry/service/i18n"
	"gusseynov/GO-Registry/storage" // Добавьте импорт вашего пакета базы данных

	go_ora "github.com/sijms/go-ora/v2"
)

// Оригинальные структуры полей СУБД Oracle
type TimeOffItem struct {
	EventDate string `db:"EVENT_DATE"`
	TimeOut   string `db:"TIME_OUT"`
	TimeIn    string `db:"TIME_IN"`
	Employee  string `db:"EMPLOYEE"`
	Post      string `db:"POST"`
	DepName   string `db:"DEP_NAME"`
	Cause     string `db:"CAUSE"`
	Head      string `db:"HEAD"`
	Status    int    `db:"STATUS"`
	TimeFact  string `db:"TIME_FACT"`
	CntDays   int    `db:"CNT_DAYS"`
	Status2   int    `db:"STATUS2"`
	ID        int    `db:"ID"`
}

type ViewTimeOff struct {
	*middleware.BasePageContext // 💡 Встраиваем базовый контекст (UserName, DepName, Lang, Theme перелетели)
	Message                     string
	ListTimeOff                 []TimeOffItem
	AllMessages                 []message.MessageItem
	// Поля совместимости со старыми переменными в шаблоне time_off.html
	Style    string
	Lang     string
	UserName string
	UserPost string
	UserDep  string
}

// TimeOffGet отображает список активных заявок сотрудника (GET /time-off)
func TimeOffGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Извлекаем готовый пакет данных из нашей объединенной мидлвари Authorize
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		// 2. ВЫЗЫВАЕМ ОРИГИНАЛЬНЫЙ SELECT ЗАПРОС К ORACLE
		query := `SELECT 
            TO_CHAR(event_date, 'YYYY-MM-DD') AS event_date, 
            TO_CHAR(time_out, 'YYYY-MM-DD HH24:MI') AS time_out, 
            TO_CHAR(time_in, 'YYYY-MM-DD HH24:MI') AS time_in, 
            employee, 
            post, 
            dep_name, 
            COALESCE(cause, ' ') AS cause, 
            COALESCE(head, ' ') AS head, 
            status, 
            COALESCE(TO_CHAR(time_fact, 'YYYY-MM-DD HH24:MI'), ' ') AS time_fact,
            TO_NUMBER(TRUNC(sysdate) - TRUNC(time_out)) AS cnt_days, 
            status AS status2, 
            id
          FROM register r
          WHERE r.employee = :1
          ORDER BY r.event_date DESC`

		var list []TimeOffItem
		// sqlx сам выполнит запрос, промапит колонки по тегам db и запишет в слайс list
		err := storage.DB.SelectContext(r.Context(), &list, query, pageCtx.FIO)
		if err != nil {
			slog.Error("Ошибка получения списка отсутствий из Oracle через sqlx", "user", pageCtx.LoginName, "err", err)
			http.Error(w, "Ошибка базы данных: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Постобработка: аккуратно подрезаем длины строк прямо в слайсе
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

		// Вычисляем сообщение об успехе, используя язык из готового pageCtx.Lang
		// 🚀 ЧИТАЕМ СООБЩЕНИЯ ДЛЯ БАЗОВОГО ШАБЛОНА И ЗДЕСЬ ТОЖЕ!
		messages := message.GetAllMessage(r.Context())

		msgParam := r.URL.Query().Get("msg")
		messageText := ""
		if msgParam == "success" {
			messageText = i18n.Get(pageCtx.Lang, "TIME_OFF_SAVED")
			if messageText == "TIME_OFF_SAVED" {
				messageText = "Запись успешно сохранена!"
			}
		}

		// 3. Формируем структуру данных. Поля подтягиваются автоматически
		data := ViewTimeOff{
			BasePageContext: pageCtx, // Внедряем базовые данные (Lang, Theme, FIO)
			Message:         messageText,
			ListTimeOff:     list,
			AllMessages:     messages,
		}

		// 4. Компиляция шаблонов с привязкой i18n на основе вычисленного языка
		tmpl, err := template.New("base.html").Funcs(template.FuncMap{
			"res_value": func(key string) string {
				return i18n.Get(pageCtx.Lang, key)
			},
		}).ParseFiles(
			"templates/base.html",
			"templates/time_off.html",
		)

		if err != nil {
			slog.Error("Ошибка компиляции шаблона time_off", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Безопасный рендеринг через буфер с явным указанием корневого base.html
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
func TimeOffPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Извлекаем готовый пакет данных из нашей мидлвари Authorize
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())
		// if pageCtx.IsAnonymous {
		// 	slog.Error("Пользователь отсутствует в контексте при отправке формы time-off")
		// 	http.Redirect(w, r, "/login", http.StatusSeeOther)
		// 	return
		// }

		// Читаем данные из формы
		dateOut := r.FormValue("date_out")
		dateIn := r.FormValue("date_in")
		cause := r.FormValue("cause")

		// Валидация входных данных
		if dateOut == "" || dateIn == "" || len(cause) < 12 {
			http.Error(w, "Заполните все поля. Длина причины должна быть не менее 12 символов.", http.StatusBadRequest)
			return
		}
		// Форматируем даты под маску YYYY-MM-DD HH24:MI, которую требует функция TO_DATE в Oracle
		formattedDateOut := strings.Replace(dateOut, "T", " ", 1)
		formattedDateIn := strings.Replace(dateIn, "T", " ", 1)

		// Извлекаем сохраненный в мидлвари IP-адрес для системных логов
		clientIP := middleware.GetIPFromContext(r.Context())
		slog.Debug("Вызов PL/SQL функции reg.add_reg через анонимный блок", "user", pageCtx.LoginName, "ip", clientIP)

		slog.Debug("TIME_OFF CHECK DATA", "dateOut", formattedDateOut, "dateIn", formattedDateIn, "cause", cause)

		// Вызов функции через именованные параметры
		// Задаем формат даты для всей сессии Oracle перед выполнением функции
		query := `BEGIN :ret_val := reg.add_reg(:d_out,:d_in, :fio, :pst, :dep, :caus); END;`

		var oracleResult string

		// Передаем параметры через sql.Named. Каждая переменная имеет четкий тип string.
		_, err := storage.DB.DB.ExecContext(r.Context(), query,
			sql.Named("ret_val", go_ora.Out{Dest: &oracleResult, Size: 256}),
			sql.Named("d_out", string(formattedDateOut)),
			sql.Named("d_in", string(formattedDateIn)),
			sql.Named("fio", string(pageCtx.FIO)),
			sql.Named("pst", string(pageCtx.Post)),
			sql.Named("dep", string(pageCtx.DepName)),
			sql.Named("caus", string(cause)),
		)

		// Обработка системных ошибок связи с Oracle
		if err != nil {
			slog.Error("Критическая ошибка выполнения анонимного блока reg.add_reg", "err", err)
			http.Error(w, "Ошибка связи с базой данных: "+err.Error(), http.StatusInternalServerError)
			return
		}

		slog.Info("Результат выполнения функции reg.add_reg", "status", oracleResult)

		// Анализируем текстовый ответ из Oracle
		slog.Info("Результат выполнения функции reg.add_reg", "status", oracleResult)

		// АНАЛИЗИРУЕМ ТЕКСТОВЫЙ ОТВЕТ ИЗ ORACLE
		if oracleResult != "Success" {
			slog.Info("TIME_OFF Отклонено базой", "USER", pageCtx.LoginName, "REJECT_REASON", oracleResult)

			// 🚀 ВМЕСТО ЧЕРНОГО ОКНА — РЕНДЕРИМ ШАБЛОН С ОШИБКОЙ!

			// 1. Быстро дозапрашиваем список документов для таблицы, чтобы она не пришла пустой
			var list []TimeOffItem
			queryGet := `SELECT TO_CHAR(event_date, 'YYYY-MM-DD') AS event_date, 
			                    TO_CHAR(time_out, 'YYYY-MM-DD HH24:MI') AS time_out, 
			                    TO_CHAR(time_in, 'YYYY-MM-DD HH24:MI') AS time_in, 
			                    employee, post, dep_name, coalesce(cause, ' ') as cause, 
			                    coalesce(head, ' ') as head, status, id 
			             FROM register WHERE employee = :1 ORDER BY event_date DESC`
			_ = storage.DB.SelectContext(r.Context(), &list, queryGet, pageCtx.FIO)

			// 2. Собираем сообщения i18n
			messages := message.GetAllMessage(r.Context())

			// 3. Формируем data и кладем ошибку Oracle прямо в Message
			data := ViewTimeOff{
				BasePageContext: pageCtx,
				Message:         oracleResult, // <-- Текст ошибки "Регистрируемая дата выхода..." уйдет в шаблон!
				ListTimeOff:     list,
				AllMessages:     messages,
			}

			// 4. Компилируем и выводим форму обратно пользователю
			tmpl, err := template.New("base.html").Funcs(template.FuncMap{
				"res_value": func(key string) string { return i18n.Get(pageCtx.Lang, key) },
			}).ParseFiles("templates/base.html", "templates/time_off.html")

			if err != nil {
				http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
				return
			}

			var buf bytes.Buffer
			_ = tmpl.ExecuteTemplate(&buf, "base.html", data)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = buf.WriteTo(w)
			return // Выходим из POST метода, не делая редирект
		}

		slog.Info("TIME_OFF Успешно создано", "USER", pageCtx.LoginName, "STATUS", oracleResult)

		// Если всё хорошо — вот тогда делаем стандартный редирект на журнал
		http.Redirect(w, r, "/time-off?msg=success", http.StatusSeeOther)
	}
}

// DelFromListTimeOff удаляет запись отсутствия через процедуру Oracle (POST /del-from-list-time-off)
func DelFromListTimeOff() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Проверяем авторизацию через единый контекст мидлвари Authorize
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())
		if pageCtx.IsAnonymous {
			slog.Error("Анонимная попытка удаления записи отсутствия")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Безопасность: разрешаем только метод POST
		if r.Method != http.MethodPost {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		// 2. Читаем ID из POST-параметров формы
		idStr := r.FormValue("id")
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			slog.Warn("Некорректный ID в POST-запросе удаления", "id_str", idStr)
			http.Error(w, "Неверный идентификатор записи", http.StatusBadRequest)
			return
		}

		// Извлекаем сохраненный в мидлвари IP-адрес для системных логов
		clientIP := middleware.GetIPFromContext(r.Context())
		slog.Info("Запрос на безопасное POST-удаление отсутствия", "user", pageCtx.LoginName, "id", id, "ip", clientIP)

		// 3. Вызываем хранимую процедуру Oracle
		query := `BEGIN reg.del_time_off(:1); END;`

		_, err = storage.DB.ExecContext(r.Context(), query, id)
		if err != nil {
			slog.Error("Критическая ошибка выполнения процедуры reg.del_time_off в Oracle через POST", "id", id, "err", err)
			http.Error(w, "Ошибка удаления в базе Oracle: "+err.Error(), http.StatusInternalServerError)
			return
		}

		slog.Info("Запись отсутствия успешно удалена из Oracle через POST", "id", id, "user", pageCtx.LoginName)

		// 4. Редиректим обратно на нашу страницу с параметром удаления
		http.Redirect(w, r, "/time-off?msg=deleted", http.StatusSeeOther)
	}
}
