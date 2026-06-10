package report

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/xuri/excelize/v2"
)

type ColumnDef struct {
	Name string
	Type string
	Len  int // "string", "int", "float", "money"
}

type ExcelBuilder struct {
	File   *excelize.File
	Styles *ExcelStyles
}

func NewExcelBuilder() (*ExcelBuilder, error) {
	f := excelize.NewFile()
	styles, err := NewExcelStyles(f)
	if err != nil {
		return nil, err
	}
	return &ExcelBuilder{File: f, Styles: styles}, nil
}

// Создать лист
func (b *ExcelBuilder) Sheet(name string) {
	b.File.NewSheet(name)
}

// Удалить дефолтный лист
func (b *ExcelBuilder) RemoveDefault() {
	_ = b.File.DeleteSheet("Sheet1")
}

// Активный лист
func (b *ExcelBuilder) SetActive(sheet string) {
	if idx, err := b.File.GetSheetIndex(sheet); err == nil {
		b.File.SetActiveSheet(idx)
	}
}

// Заголовок
func (b *ExcelBuilder) WriteTitle(sheet, title string) {
	b.File.SetCellValue(sheet, "A1", title)
	b.File.SetCellStyle(sheet, "A1", "A1", b.Styles.TitleName)
}

// Подзаголовок
func (b *ExcelBuilder) WriteSubTitle(sheet, text string) {
	b.File.SetCellValue(sheet, "A2", text)
	b.File.SetCellStyle(sheet, "A2", "A2", b.Styles.TitleName)
}

// Шапка таблицы
func (b *ExcelBuilder) WriteHeader(sheet string, row int, headers []string) {
	for i, h := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		cell := fmt.Sprintf("%s%d", col, row)
		b.File.SetCellValue(sheet, cell, h)
		b.File.SetCellStyle(sheet, cell, cell, b.Styles.TitleHeader)
	}
}

// SQL-блок
func (b *ExcelBuilder) WriteSQL(sheet string, sql string) {
	_ = b.File.MergeCell(sheet, "A1", "I25")
	_ = b.File.SetCellValue(sheet, "A1", sql)
	_ = b.File.SetCellStyle(sheet, "A1", "I25", b.Styles.SQLBlock)
}

// Запись строки
func (b *ExcelBuilder) WriteRow(sheet string, rowIndex int, values []any, styles []int) {
	for i, v := range values {
		col, _ := excelize.ColumnNumberToName(i + 1)
		cell := fmt.Sprintf("%s%d", col, rowIndex)
		b.File.SetCellValue(sheet, cell, v)
		if styles != nil && i < len(styles) {
			b.File.SetCellStyle(sheet, cell, cell, styles[i])
		}
	}
}

// Freeze шапки
func (b *ExcelBuilder) FreezeHeader(sheet string, headerRow int) {
	_ = b.File.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       true,
		XSplit:      0,
		YSplit:      headerRow,
		TopLeftCell: fmt.Sprintf("A%d", headerRow+1),
	})
}

// Автоширина
func (b *ExcelBuilder) AutoWidth(sheet string, fromCol, toCol string, maxRow int) {
	fromIdx, _ := excelize.ColumnNameToNumber(fromCol)
	toIdx, _ := excelize.ColumnNameToNumber(toCol)

	for colIdx := fromIdx; colIdx <= toIdx; colIdx++ {
		colName, _ := excelize.ColumnNumberToName(colIdx)
		maxWidth := 8.0

		for row := 1; row <= maxRow; row++ {
			cell := fmt.Sprintf("%s%d", colName, row)
			val, _ := b.File.GetCellValue(sheet, cell)
			if val == "" {
				continue
			}
			l := float64(utf8.RuneCountInString(val))
			if l > maxWidth {
				maxWidth = l
			}
		}
		_ = b.File.SetColWidth(sheet, colName, colName, maxWidth+2)
	}
}

// Фильтр
func (b *ExcelBuilder) AutoFilter(sheet string, fromCol, toCol string, headerRow int) {
	ref := fmt.Sprintf("%s%d:%s%d", fromCol, headerRow, toCol, headerRow)
	_ = b.File.AutoFilter(sheet, ref, nil)
}

// Сохранение
func (b *ExcelBuilder) Save(path string) error {
	return b.File.SaveAs(path)
}

func (b *ExcelBuilder) SetColumnWidth(sheet, col string, width float64) {
	_ = b.File.SetColWidth(sheet, col, col, width)
}

func (b *ExcelBuilder) SetColumnWidths(sheet string, widths map[string]float64) {
	for col, w := range widths {
		_ = b.File.SetColWidth(sheet, col, col, w)
	}
}

func ConvertValue(v any, typ string) any {
	if v == nil {
		return ""
	}

	switch typ {

	// -----------------------------
	// STRING TYPES
	// -----------------------------
	case "string", "string-left", "string-center", "string-right":
		return fmt.Sprintf("%v", v)

	// -----------------------------
	// DATE (строка dd.mm.yyyy)
	// -----------------------------
	case "date":
		return fmt.Sprintf("%v", v)

	// -----------------------------
	// DATETIME (строка dd.mm.yyyy hh:mm)
	// -----------------------------
	case "datetime":
		return fmt.Sprintf("%v", v)

	// -----------------------------
	// INTEGER
	// -----------------------------
	case "int":
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case int32:
			return int(val)
		case float64:
			return int(val)
		case float32:
			return int(val)
		case uint:
			return int(val)
		case uint64:
			return int(val)
		case uint32:
			return int(val)
		case []uint8:
			n, _ := strconv.Atoi(string(val))
			return n
		default:
			// ЛЮБОЙ другой тип — пробуем через fmt
			i, err := strconv.Atoi(fmt.Sprintf("%v", v))
			if err == nil {
				return i
			}
			return 0
		}
	// -----------------------------
	// FLOAT
	// -----------------------------
	case "float":
		switch val := v.(type) {
		case float64:
			return val
		case []uint8:
			f, _ := strconv.ParseFloat(string(val), 64)
			return f
		default:
			return 0.0
		}

	// -----------------------------
	// MONEY
	// -----------------------------
	case "money":
		switch val := v.(type) {
		case float64:
			return val
		case []uint8:
			f, _ := strconv.ParseFloat(string(val), 64)
			return f
		default:
			return 0.0
		}
	}

	return fmt.Sprintf("%v", v)
}
