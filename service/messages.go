// service/message/message.go
package service

import (
	"context"

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
}

// GetAllMessage вытаскивает сообщения за последние 8 дней из Oracle.
// Гарантированно возвращает пустой слайс []MessageItem{} вместо nil, защищая UI от падений.
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
            COALESCE(message, ' ') AS message
        FROM messages
        WHERE mess_date > sysdate - 8
        ORDER BY mess_date DESC`

	err := storage.DBSelectMany(ctx, "list_messages", &list, query)
	if err != nil {
		return []MessageItem{}
	}

	return list
}
