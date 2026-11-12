// service/list_all_off_json.go
package service

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"gusseynov/GO-Registry/storage" // Пакет с пулом соединений к Oracle (storage.DB)
)

// TimeOffJsonItem — строка выгрузки входа-выхода.
// Своя структура нужна, чтобы service не импортировал web:
// web уже импортирует service, обратный импорт даёт цикл.
type TimeOffJsonItem struct {
	EventDate string `db:"EVENT_DATE" json:"event_date"`
	TimeOut   string `db:"TIME_OUT"   json:"time_out"`
	TimeIn    string `db:"TIME_IN"    json:"time_in"`
	Employee  string `db:"EMPLOYEE"   json:"employee"`
	Post      string `db:"POST"       json:"post"`
	DepName   string `db:"DEP_NAME"   json:"dep_name"`
	Cause     string `db:"CAUSE"      json:"cause"`
	Head      string `db:"HEAD"       json:"head"`
	Status    int    `db:"STATUS"     json:"status"`
	TimeFact  string `db:"TIME_FACT"  json:"time_fact"`
	CntDays   int    `db:"CNT_DAYS"   json:"cnt_days"`
	Status2   int    `db:"STATUS2"    json:"status2"`
	ID        int    `db:"ID"         json:"id"`
}

// ListTimeOffJson отдаёт статистику входа-выхода за месяц (month=YYYY-MM) в формате JSON, по всем подразделениям.
func ListTimeOffJson() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Формат YYYY-MM (например 2026-11); работает и для GET (?month=...), и для POST.
		selectedMonth := time.Now().Format("2006-01")
		if formMonth := r.FormValue("month"); formMonth != "" {
			selectedMonth = formMonth
		}

		// Вызывает сервис, а не пользователь: фильтра по подразделению нет,
		// доступ ограничен списком адресов (web.RequireExportAccess).
		whereClause := "WHERE TRUNC(event_date, 'MM') = TO_DATE(:1, 'YYYY-MM')"

		query := fmt.Sprintf(`SELECT
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
				  %s
				  ORDER BY r.event_date DESC`, whereClause)

		list := []TimeOffJsonItem{} // пустой результат -> [], а не null
		if err := storage.DBSelectMany(r.Context(), "all_list_json", &list, query, selectedMonth); err != nil {
			http.Error(w, "Ошибка базы данных: "+err.Error(), http.StatusInternalServerError)
			return
		}
		slog.Info("ListTimeOffJson: retrieved list", "count", len(list))

		WriteJSON(w, list)
	}
}
