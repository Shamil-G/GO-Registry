package report

import "github.com/xuri/excelize/v2"

func StyleForType(excel *ExcelBuilder, typ string) int {
	switch typ {
	case "string":
		return excel.Styles.StringLeft
	case "string-left":
		return excel.Styles.StringLeft
	case "string-center":
		return excel.Styles.StringCenter
	case "string-right":
		return excel.Styles.StringRight
	case "int":
		return excel.Styles.DigitalCenter
	case "float":
		return excel.Styles.DigitalRight
	case "money":
		return excel.Styles.Money
	case "date":
		return excel.Styles.Date
	case "datetime":
		return excel.Styles.DateTime
	default:
		return excel.Styles.StringLeft
	}
}

// Общие рамки
func borderThin() []excelize.Border {
	return []excelize.Border{
		{Type: "top", Style: 1, Color: "000000"},
		{Type: "bottom", Style: 1, Color: "000000"},
		{Type: "left", Style: 1, Color: "000000"},
		{Type: "right", Style: 1, Color: "000000"},
	}
}

func borderThick() []excelize.Border {
	return []excelize.Border{
		{Type: "top", Style: 6, Color: "000000"},
		{Type: "bottom", Style: 6, Color: "000000"},
		{Type: "left", Style: 6, Color: "000000"},
		{Type: "right", Style: 6, Color: "000000"},
	}
}

// Вспомогательная функция
func strPtr(s string) *string { return &s }

// Фабрика стилей
type ExcelStyles struct {
	TitleHeader   int
	TitleName     int
	ReportCode    int
	TimeStamp     int
	Date          int
	DateTime      int
	StringCenter  int
	StringLeft    int
	StringRight   int
	Digital       int
	DigitalRight  int
	DigitalCenter int
	DigitalBold   int
	Money         int
	SQLBlock      int
}

func NewExcelStyles(f *excelize.File) (*ExcelStyles, error) {

	titleHeader, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"D1FFFF"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    borderThin(),
	})

	titleName, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14},
	})

	reportCode, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "right"},
	})

	timeStamp, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Italic: true},
		Alignment: &excelize.Alignment{Horizontal: "right"},
	})

	stringLeft, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
		Border: borderThin(),
	})

	stringCenter, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: borderThin(),
	})
	stringRight, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "right",
			Vertical:   "center",
		},
		Border: borderThin(),
	})

	digital, _ := f.NewStyle(&excelize.Style{
		Alignment:    &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:       borderThin(),
		CustomNumFmt: strPtr("# ### ### ##0"),
	})

	sqlBlock, _ := f.NewStyle(&excelize.Style{
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FAFAD7"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "top", WrapText: true},
		Border:    borderThick(),
	})
	// DigitalRight — числа, выравнивание вправо
	digitalRight, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "right",
			Vertical:   "center",
		},
		Border: borderThin(),
		NumFmt: 1, // обычный числовой формат
	})

	// DigitalCenter — числа, выравнивание по центру
	digitalCenter, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: borderThin(),
		NumFmt: 1,
	})

	// DigitalBold — жирные числа
	digitalBold, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: borderThin(),
		NumFmt: 1,
	})

	// Money — денежный формат
	money, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "right",
			Vertical:   "center",
		},
		Border: borderThin(),
		NumFmt: 4, // Excel built‑in: "#,##0.00"
	})

	dateStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: borderThin(),
		NumFmt: 14, // dd.mm.yyyy
	})

	dateTimeStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Border: borderThin(),
		NumFmt: 22, // dd.mm.yyyy hh:mm
	})

	return &ExcelStyles{
		TitleHeader:  titleHeader,
		TitleName:    titleName,
		ReportCode:   reportCode,
		TimeStamp:    timeStamp,
		Date:         dateStyle,
		DateTime:     dateTimeStyle,
		StringLeft:   stringLeft,
		StringCenter: stringCenter,
		StringRight:  stringRight,
		Digital:      digital,
		SQLBlock:     sqlBlock,

		// новые стили:
		DigitalRight:  digitalRight,
		DigitalCenter: digitalCenter,
		DigitalBold:   digitalBold,
		Money:         money,
	}, nil
}
