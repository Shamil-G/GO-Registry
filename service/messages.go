// service/message/message.go
package service

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"

	// Подключите ваш пакет с пулом соединений к Oracle (например, storage)
	"gusseynov/GO-Registry/storage"
)

// MessageItem описывает структуру сообщения из базы данных Oracle
type MessageItem struct {
	IDMessage int    `json:"id_mess"`
	Date      string `json:"mess_date"`
	Author    string `json:"author"`
	DepName   string `json:"dep_name"`
	Message   string `json:"message"`
}

// GetAllMessage вытаскивает сообщения за последние 8 дней из Oracle.
// Гарантированно возвращает пустой слайс []MessageItem{} вместо nil, защищая UI от падений.
func GetAllMessage(ctx context.Context) []MessageItem {
	listMessage := []MessageItem{} // Инициализируем пустой сейфовый слайс []

	query := `
		SELECT id_mess, mess_date, author, dep_name, message 
		FROM messages 
		WHERE mess_date > sysdate - 8
		ORDER BY mess_date DESC`

	rows, err := storage.DB.QueryContext(ctx, query)
	if err != nil {
		slog.Error("Критическая ошибка получения сообщений из Oracle", "err", err)
		return listMessage
	}
	defer rows.Close()

	for rows.Next() {
		var item MessageItem
		var rawID sql.NullInt64
		var rawDate, rawAuthor, rawDep, rawMsg sql.NullString

		err := rows.Scan(&rawID, &rawDate, &rawAuthor, &rawDep, &rawMsg)
		if err != nil {
			slog.Error("Ошибка сканирования строки messages в сервисе", "err", err)
			continue
		}

		// 1. Форматируем дату до 16 символов (аналог Python: str(row)[0:16])
		item.Date = rawDate.String
		if len(item.Date) > 16 {
			item.Date = item.Date[:16]
		}

		// 2. Форматируем автора до короткого ФИО (аналог Python: "И. Иванов")
		authorName := rawAuthor.String
		if authorName != "" {
			splitName := strings.Fields(authorName)
			if len(splitName) >= 2 {
				// Безопасно берем первую букву имени с учетом UTF-8 (руны)
				firstNameRunes := []rune(splitName[1])
				var firstInitial string
				if len(firstNameRunes) > 0 {
					firstInitial = string(firstNameRunes[0])
				}
				item.Author = firstInitial + ". " + splitName[0]
			} else {
				item.Author = authorName
			}
		}

		item.IDMessage = int(rawID.Int64)
		item.DepName = rawDep.String
		if item.DepName == "" {
			item.DepName = " "
		}
		item.Message = rawMsg.String

		listMessage = append(listMessage, item)
	}

	slog.Debug("Бизнес-логика: Сообщения успешно загружены", "count", len(listMessage))
	return listMessage
}
