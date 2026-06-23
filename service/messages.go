// service/message/message.go
package service

import (
	"context"
	"log/slog"
	"strings"

	// Подключите ваш пакет с пулом соединений к Oracle (например, storage)
	"gusseynov/GO-Registry/storage"
)

// MessageItem описывает структуру сообщения из базы данных Oracle
type MessageItem struct {
	IDMessage int    `db:"ID_MESS" json:"id_mess"`
	Date      string `db:"MESS_DATE" json:"mess_date"`
	Author    string `db:"AUTHOR" json:"author"`
	DepName   string `db:"DEP_NAME" json:"dep_name"`
	Message   string `db:"MESSAGE" json:"message"`
	// ИСПРАВЛЕНО: Добавлены новые поля, чтобы маппинг из DBSelectMany отработал без ошибок
	PhotoURL  string `db:"PHOTO_URL" json:"photo_url"`
	PhotoFIO  string `db:"PHOTO_FIO" json:"photo_fio"`
	PhotoPost string `db:"PHOTO_POST" json:"photo_post"`
}

// GetAllMessage вытаскивает сообщения за последние 8 дней из Oracle.

// GetAllMessage вытаскивает сообщения за последние 8 дней из Oracle.
func GetAllMessage(ctx context.Context) []MessageItem {
	list := []MessageItem{}

	query := `
		SELECT
			id_mess,
			SUBSTR(TO_CHAR(mess_date, 'DD.MM.YYYY HH24:MI'), 1, 16) AS mess_date,
			CASE
				WHEN author IS NULL THEN ' '
				ELSE
					SUBSTR(author, INSTR(author, ' ') + 1, 1) || '. ' ||
					SUBSTR(author, 1, INSTR(author, ' ') - 1)
			END AS author,
			COALESCE(dep_name, ' ') AS dep_name,
			COALESCE(message, ' ') AS message,
			
			-- ИСПРАВЛЕНИЕ: Возвращаем пробел ' ', чтобы избежать NULL в драйвере Go
			COALESCE(PHOTO_URL, ' ') AS PHOTO_URL,
			COALESCE(PHOTO_FIO, ' ') AS PHOTO_FIO,
			COALESCE(PHOTO_POST, ' ') AS PHOTO_POST
		FROM messages
		WHERE mess_date > sysdate - 3
		ORDER BY mess_date DESC`

	storage.DBSelectMany(ctx, "list_messages", &list, query)

	if len(list) == 0 {
		slog.Warn("Слайс сообщений пуст или произошла внутренняя ошибка в storage")
		return []MessageItem{}
	}

	// Шаг 2: Обрезаем пробелы внутри Go, чтобы шаблонизатор {{ if .PhotoURL }} работал корректно
	for i := range list {
		// 1. Сначала очищаем от пробелов
		photoURL := strings.TrimSpace(list[i].PhotoURL)

		// 2. Добавляем префикс только если строка НЕ пустая
		if photoURL != "" {
			list[i].PhotoURL = "static/photos/" + photoURL
		} else {
			list[i].PhotoURL = ""
		}
		list[i].PhotoFIO = strings.TrimSpace(list[i].PhotoFIO)
		list[i].PhotoPost = strings.TrimSpace(list[i].PhotoPost)
		list[i].Author = strings.TrimSpace(list[i].Author)
		list[i].DepName = strings.TrimSpace(list[i].DepName)
		list[i].Message = strings.TrimSpace(list[i].Message)
	}

	slog.Debug("Сообщения успешно получены", "count", len(list))
	return list
}
