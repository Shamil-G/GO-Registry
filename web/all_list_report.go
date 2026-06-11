package web

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gusseynov/GO-Registry/middleware"
	"gusseynov/GO-Registry/report"
)

// AllListReportHandler генерирует Excel отчет и отправляет его пользователю на скачивание
func AllListReport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Проверяем авторизацию через мидлварь
		ssoUser := middleware.GetOrCreatePageCtx(r.Context())

		// 2. Извлекаем месяц фильтрации из URL параметров (?flt_month=2026-06)
		fltMonth := r.URL.Query().Get("flt_month")

		// Если месяц не передан в URL, берем текущий системный месяц
		if fltMonth == "" {
			fltMonth = time.Now().Format("2006-01")
		}

		// Формируем имя файла (например: report_01_2026-06.xlsx)
		fileName := fmt.Sprintf("rep_all_list_%s.xlsx", fltMonth)

		slog.Info("Запрос на генерацию Excel отчета", "user", ssoUser.FIO, "month", fltMonth, "file", fileName)

		// 3. Запускаем генерацию Excel-файла через наш модуль report
		fullPath, err := report.DoAllListTimeOffReport(r.Context(), fltMonth, fileName)
		if err != nil {
			slog.Error("Критическая ошибка генерации Excel отчета", "month", fltMonth, "err", err)
			http.Error(w, "Ошибка формирования отчета: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Проверяем физическое существование созданного файла на диске
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			slog.Error("Сгенерированный файл отчета не найден на диске", "path", fullPath)
			http.Error(w, "Файл отчета не найден", http.StatusInternalServerError)
			return
		}

		// 4. НАСТРАИВАЕМ HTTP-ЗАГОЛОВКИ ДЛЯ ПРИНУДИТЕЛЬНОГО СКАЧИВАНИЯ (File Download)
		// Указываем тип контента как бинарный поток данных
		w.Header().Set("Content-Type", "application/octet-stream")
		// Заголовок Content-Disposition заставляет браузер открыть окно сохранения файла вместо попытки отобразить его
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(fullPath)))

		// Отправляем файл пользователю
		http.ServeFile(w, r, fullPath)
	}
}
