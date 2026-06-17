package report

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gusseynov/GO-Registry/middleware"
	"gusseynov/GO-Registry/storage"

	"github.com/xuri/excelize/v2"
)

type TimeOffRow struct {
	EventDate string `db:"EVENT_DATE"`
	TimeOut   string `db:"TIME_OUT"`
	TimeIn    string `db:"TIME_IN"`
	Employee  string `db:"EMPLOYEE"`
	Post      string `db:"POST"`
	DepName   string `db:"DEP_NAME"`
	Cause     string `db:"CAUSE"`
	Head      string `db:"HEAD"`
	Status    int    `db:"STATUS"`
}

func DoAllListTimeOffReport(ctx context.Context, fltMonth, fileName string) (string, error) {
	fullPath := filepath.Join("./reports", fileName)
	_ = os.Remove(fullPath)

	excel, _ := NewExcelBuilder()
	excel.Sheet("Список")
	excel.Sheet("SQL")
	excel.RemoveDefault()
	excel.SetActive("Список")

	pageCtx := middleware.GetOrCreatePageCtx(ctx)
	whereClause := "WHERE TRUNC(event_date, 'MM') = TO_DATE(:1, 'YYYY-MM')"
	// 3. ДОБАВЛЕНИЕ ПРЕФИКСОВ (ФИЛЬТРОВ)
	if !pageCtx.IsAdmin {
		if len(pageCtx.SubordinateOU) > 0 {
			inList := strings.Join(pageCtx.SubordinateOU, "', '")
			whereClause += fmt.Sprintf(" AND dep_name IN ('%s')", inList)
		} else {
			whereClause += fmt.Sprintf(" AND dep_name = '%s'", pageCtx.DepName)
		}
	}
	// SQL лист
	query := fmt.Sprintf(`
        SELECT 
            TO_CHAR(event_date, 'DD.MM.YYYY') AS event_date,
            TO_CHAR(time_out, 'DD.MM.YYYY HH24:MI') AS time_out,
            TO_CHAR(time_in, 'DD.MM.YYYY HH24:MI') AS time_in,
            employee, post, dep_name,
            COALESCE(cause, ' ') AS cause,
            COALESCE(head, ' ') AS head,
            status
        FROM register
		%s
        ORDER BY dep_name, employee, event_date
    `, whereClause)

	// 4. Финальный SELECT запрос к Oracle
	excel.WriteSQL("SQL", query)

	// Заголовки
	excel.WriteTitle("Список", "Список зарегистрированных выходов с работы")
	excel.WriteSubTitle("Список", "За период: "+fltMonth)

	// ------------------------------------------------------------
	// Формируем список колонок c типами и длинами
	// ------------------------------------------------------------

	cols := []ColumnDef{
		{Name: "№", Type: "int", Len: 6},
		{Name: "Дата регистрации", Type: "string-center", Len: 12},
		{Name: "Время выхода", Type: "string-center", Len: 17},
		{Name: "Время прихода", Type: "string-center", Len: 17},
		{Name: "Сотрудник", Type: "string", Len: 40},
		{Name: "Должность", Type: "string", Len: 32},
		{Name: "Департамент", Type: "string", Len: 45},
		{Name: "Причина отсутствия", Type: "string", Len: 45},
		{Name: "ФИО руководителя", Type: "string", Len: 32},
		{Name: "STATUS", Type: "string", Len: 20},
	}

	// ------------------------------------------------------------
	// Пишем шапку
	// ------------------------------------------------------------

	var headers []string
	for _, c := range cols {
		headers = append(headers, c.Name)
	}

	excel.WriteHeader("Список", 3, headers)
	excel.FreezeHeader("Список", 3)

	// Устанавливаем ширины
	for i, col := range cols {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		excel.SetColumnWidth("Список", colName, float64(col.Len))
	}
	// ------------------------------------------------------------
	// ДАННЫЕ
	// ------------------------------------------------------------

	// Данные
	var rows []TimeOffRow
	if err := storage.DB.SelectContext(ctx, &rows, query, fltMonth); err != nil {
		return "", err
	}

	rowIndex := 4

	for i, r := range rows {
		status := map[int]string{
			0: "На согласовании",
			1: "Согласовано",
			2: "Отказано",
		}[r.Status]

		rowMap := map[string]any{
			"№":                  i + 1,
			"Дата регистрации":   r.EventDate,
			"Время выхода":       r.TimeOut,
			"Время прихода":      r.TimeIn,
			"Сотрудник":          r.Employee,
			"Должность":          r.Post,
			"Департамент":        r.DepName,
			"Причина отсутствия": r.Cause,
			"ФИО руководителя":   r.Head,
			"STATUS":             status,
		}

		values := []any{}
		styles := []int{}
		// slog.Debug("[PRINT COLS]", "rowMap", rowMap)
		for _, col := range cols {
			raw := rowMap[col.Name]
			values = append(values, ConvertValue(raw, col.Type))
			styles = append(styles, StyleForType(excel, col.Type))
		}

		excel.WriteRow("Список", rowIndex, values, styles)
		rowIndex++
	}

	// Служебная информация
	lastCol, _ := excelize.ColumnNumberToName(len(cols))
	now := time.Now()

	excel.File.SetCellValue("Список", lastCol+"1", "TIMEOFF")
	excel.File.SetCellStyle("Список", lastCol+"1", lastCol+"1", excel.Styles.ReportCode)

	excel.File.SetCellValue("Список", lastCol+"2",
		fmt.Sprintf("Дата формирования: %s (%s)",
			now.Format("02.01.2006"),
			now.Format("15:04:05"),
		),
	)
	excel.File.SetCellStyle("Список", lastCol+"2", lastCol+"2", excel.Styles.TimeStamp)

	// Сохранение
	if err := excel.Save(fullPath); err != nil {
		return "", err
	}

	slog.Info("Отчёт сформирован", "file", fullPath)
	return fullPath, nil
}
