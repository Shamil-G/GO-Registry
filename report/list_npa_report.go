package report

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"gusseynov/GO-Registry/service"
	"gusseynov/GO-Registry/storage"

	"github.com/xuri/excelize/v2"
)

// ------------------------------------------------------------
// SQL builder
// ------------------------------------------------------------

func buildListNPAPivotSQL() string {
	npa := service.GetStaticNpaList()

	var cols []string
	for _, el := range npa {
		cols = append(cols, fmt.Sprintf("'%s' AS \"%s\"", el.Name, el.Name))
	}

	return fmt.Sprintf(`
        SELECT *
        FROM (
            SELECT dep_name, user_name, file_name
            FROM use_doc
            WHERE EXTRACT(YEAR FROM date_op) = :year
        )
        PIVOT(
            COUNT(file_name)
            FOR file_name IN (%s)
        )
        ORDER BY dep_name, user_name
    `, strings.Join(cols, ","))
}

// ------------------------------------------------------------
// MAIN REPORT
// ------------------------------------------------------------

func ReportUseNpa(ctx context.Context, year string) (string, error) {
	fileName := fmt.Sprintf("./reports/USE_NPA_%s.xlsx", year)
	_ = os.Remove(fileName)

	// Excel builder
	excel, _ := NewExcelBuilder()
	excel.Sheet("Список")
	excel.Sheet("SQL")
	excel.RemoveDefault()
	excel.SetActive("Список")

	// SQL лист
	sqlText := buildListNPAPivotSQL()
	excel.WriteSQL("SQL", sqlText)

	// Заголовки
	excel.WriteTitle("Список", "Сведения о количестве обращений к НПА")
	excel.WriteSubTitle("Список", "За период: "+year)

	// NPA список
	npa := service.GetStaticNpaList()

	// ------------------------------------------------------------
	// Формируем список колонок c типами и длинами
	// ------------------------------------------------------------

	cols := []ColumnDef{
		{Name: "№", Type: "int", Len: 6},
		{Name: "DEP_NAME", Type: "string", Len: 68},
		{Name: "USER_NAME", Type: "string", Len: 40},
	}

	for _, el := range npa {
		cols = append(cols, ColumnDef{
			Name: el.Name,
			Type: "int",
			Len:  12,
		})
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

	rows, err := storage.DB.QueryxContext(ctx, sqlText, year)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	rowIndex := 4

	for rows.Next() {
		rowMap := make(map[string]interface{})
		if err := rows.MapScan(rowMap); err != nil {
			slog.Error("MapScan error", "err", err)
			continue
		}

		normalized := make(map[string]any)
		for k, v := range rowMap {
			normalized[strings.ToUpper(k)] = v
		}

		// Добавляем номер строки
		normalized["№"] = rowIndex - 3

		values := []any{}
		styles := []int{}

		for _, col := range cols {
			key := strings.ToUpper(col.Name)
			value := normalized[key]

			slog.Warn("LOOKUP",
				"col", col.Name,
				"lookup_key", key,
				"value", value,
				"type", col.Type,
			)
			convert_value := ConvertValue(value, col.Type)
			values = append(values, convert_value)
			convert_style := StyleForType(excel, col.Type)
			styles = append(styles, convert_style)
			slog.Debug("[COVERT DATA]", "CONVERT_VALUE", convert_value, "CONVERT_STYLE", convert_style, "VALUES", values)
			// values = append(values, ConvertValue(value, col.Type))
			// styles = append(styles, StyleForType(excel, col.Type))
		}

		excel.WriteRow("Список", rowIndex, values, styles)
		rowIndex++
	}

	// ------------------------------------------------------------
	// СЛУЖЕБНАЯ ИНФОРМАЦИЯ
	// ------------------------------------------------------------

	infoCol, _ := excelize.ColumnNumberToName(len(cols))
	now := time.Now()

	excel.File.SetCellValue("Список", infoCol+"1", "UNPA")
	excel.File.SetCellStyle("Список", infoCol+"1", infoCol+"1", excel.Styles.ReportCode)

	excel.File.SetCellValue("Список", infoCol+"2",
		fmt.Sprintf("Дата формирования: %s (%s)",
			now.Format("02.01.2006"),
			now.Format("15:04:05"),
		),
	)
	excel.File.SetCellStyle("Список", infoCol+"2", infoCol+"2", excel.Styles.TimeStamp)

	// ------------------------------------------------------------
	// СОХРАНЕНИЕ
	// ------------------------------------------------------------

	if err := excel.Save(fileName); err != nil {
		return "", err
	}

	slog.Info("Отчёт сформирован", "file", fileName)
	return fileName, nil
}
