// service/phone_book.go
package service

import (
	"log/slog"
	"net/http"
	"os"

	"gusseynov/GO-Registry/config"
)

// PhoneBookFile отдаёт телефонный справочник целым файлом (PDF), как он лежит
// по пути PHONE_BOOK. Справочник собирается вручную на основе данных Asterisk,
// поэтому его не разбираем: любой разбор PDF хрупок, а целый файл ошибиться
// в телефонах не может. ServeFile сам ставит Last-Modified и ETag и понимает
// If-Modified-Since / Range, так что вызывающий сервис может не качать файл
// заново, пока справочник не обновили.
//
// Доступ ограничен списком адресов (web.RequireExportAccess), как у
// /all_list_off_json.
func PhoneBookFile() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := config.PhoneBook
		if path == "" {
			http.Error(w, "Справочник не настроен", http.StatusNotFound)
			return
		}
		if _, err := os.Stat(path); err != nil {
			slog.Warn("PhoneBookFile: файл справочника недоступен", "path", path, "err", err)
			http.Error(w, "Справочник недоступен", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `inline; filename="PhoneRefer.pdf"`)
		http.ServeFile(w, r, path)
	}
}
