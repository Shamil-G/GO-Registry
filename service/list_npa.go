// service/statistic.go
package service

import (
	"context"
	"log/slog"
	"time"

	"gusseynov/GO-Registry/storage" // Ваш пакет с пулом соединений к Oracle (storage.DB)
)

// NpaItem описывает один нормативно-правовой акт
type NpaItem struct {
	Num  int    `json:"num"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// GetStaticNpaList возвращает ваш оригинальный список законов
func GetStaticNpaList() []NpaItem {
	return []NpaItem{
		{Num: 1, Name: "Социальный кодекс", Path: "/static/nsi/Социальный кодекс.pdf"},
		{Num: 2, Name: "Правила исчисления, назначения 0701 от 01-01-2025", Path: "/static/nsi/Правила исчисления, назначения 0701 от 01-01-2025.pdf"},
		{Num: 3, Name: "Правила исчисления, назначения 0702 от 01-01-2025", Path: "/static/nsi/Правила исчисления, назначения 0702 от 01-01-2025.pdf"},
		{Num: 4, Name: "Правила исчисления, назначения 0703 от 01-01-2025", Path: "/static/nsi/Правила исчисления, назначения 0703 от 01-01-2025.pdf"},
		{Num: 5, Name: "Правила исчисления, назначения 0704-0705 от 01-01-2025", Path: "/static/nsi/Правила исчисления, назначения 0704-0705 от 01-01-2025.pdf"},
	}
}

// UseFileStatistic — точный перенос вашей Flask-функции use_file_statistic на Go.
// Вызывает оригинальную процедуру reg.use_file_statistic в Oracle.
// Убрали первый аргумент ctx context.Context, теперь тут ровно 4 строки!
func UseFileStatistic(userName, depName, fileName, filePath string) {
	// Создаем контекст таймаута прямо здесь внутри, автономно!
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// query := `BEGIN reg.use_file_statistic(:1, :2, :3, :4); END;`

	err := storage.DBExec(ctx, "reg.use_file_statistic", userName, depName, fileName, filePath)
	if err != nil {
		return
	}

	slog.Debug("ADD USE_FILE_STATISTIC", "employee", userName, "file_name", fileName)
}
