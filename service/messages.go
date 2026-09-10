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

	// AuthorFull — ФИО автора как оно лежит в messages.author, без сокращения
	// до инициалов. Author выше показывается человеку ("И. Иванов") и для
	// сравнения не годится: разные люди с одной фамилией и инициалом дали бы
	// одинаковую строку. Наружу не отдаём (json:"-") — в ленте оно не нужно.
	AuthorFull string `db:"AUTHOR_FULL" json:"-"`

	// CanDelete — показывать ли этому читателю кнопку "удалить". Считается в
	// Go (см. GetAllMessage), из БД не приходит. Это ТОЛЬКО про кнопку:
	// настоящая проверка живёт на сервере в web.DelMessagePost и в самой
	// reg.del_message — разметка правами не управляет.
	CanDelete bool `db:"-" json:"-"`
}

// GetAllMessage вытаскивает сообщения новостной ленты за последние 3 дня.
//
// viewerFIO — ФИО того, кто смотрит ленту (page.FIO); для анонима пустая
// строка. Нужен, чтобы проставить CanDelete у его собственных новостей.
func GetAllMessage(ctx context.Context, viewerFIO string) []MessageItem {
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
			COALESCE(author, ' ') AS author_full,
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
		list[i].AuthorFull = strings.TrimSpace(list[i].AuthorFull)
		// Своя новость — та, у которой author совпадает с ФИО читателя:
		// именно это значение web.NewMessagePost кладёт в messages.author.
		// Пустой viewerFIO (аноним) не должен совпасть ни с чем.
		list[i].CanDelete = viewerFIO != "" && strings.EqualFold(list[i].AuthorFull, strings.TrimSpace(viewerFIO))
		list[i].DepName = strings.TrimSpace(list[i].DepName)
		list[i].Message = strings.TrimSpace(list[i].Message)
	}

	slog.Debug("Сообщения успешно получены", "count", len(list))
	return list
}

// MessageOwner возвращает автора новости и имя файла её фотографии — ровно то,
// что лежит в messages.author и messages.photo_url (photo_url — только имя
// файла, без каталога: префикс static/photos/ подставляется при чтении ленты).
//
// Нужна для серверной проверки владения в web.DelMessagePost. Читать автора из
// БД, а не верить форме, обязательно: id новости приходит скрытым полем, и
// поправить его в инспекторе браузера на чужой — минутное дело. Кнопка в ленте
// (MessageItem.CanDelete) — это удобство, а правами распоряжается вот эта
// проверка. Тот же урок, что был на GO-SRM с цепочкой write-хендлеров.
//
// found=false означает, что новости с таким id нет: обычно это устаревшая
// вкладка — новость уже удалили, а страница у человека осталась открытой.
func MessageOwner(ctx context.Context, idMess int) (author, photoURL string, found bool, err error) {
	// MAX(...), а не просто колонки: агрегат возвращает ровно одну строку и
	// при отсутствии новости, то есть sql.ErrNoRows здесь не бывает вовсе.
	// Без этого «новость уже удалили» приходило бы в DBSelectOne как ошибка и
	// каждый раз писало ERROR в registry.log — шум на штатном сценарии.
	// id_mess — первичный ключ, так что агрегат берётся максимум по одной строке.
	const query = `
		SELECT COALESCE(MAX(author), ' ') || CHR(1) || COALESCE(MAX(photo_url), ' ')
		  FROM messages
		 WHERE id_mess = :1`

	var row string
	if err = storage.DBSelectOne(ctx, "message_owner", &row, query, idMess); err != nil {
		return "", "", false, err
	}

	// CHR(1) как разделитель, а не два запроса и не отдельный DBSelectOne на
	// каждую колонку: в storage есть только скалярный DBSelectOne, а заводить
	// ради двух полей выборку в структуру дороже, чем склеить их в БД.
	// В ФИО и имени файла управляющего символа быть не может.
	author, photoURL, _ = strings.Cut(row, "\x01")

	author = strings.TrimSpace(author)
	photoURL = strings.TrimSpace(photoURL)
	return author, photoURL, author != "", nil
}

// PhotoUsedByOthers — ссылается ли на этот файл ещё хоть одна новость.
//
// Имя файла собирается из ФИО на снимке (web.NewMessagePost), поэтому одна и
// та же карточка переиспользуется в разных новостях про одного человека.
// Удалить файл вместе с новостью можно только когда он больше никому не нужен,
// иначе у соседних новостей оборвётся картинка.
//
// Вызывать ПОСЛЕ удаления записи: тогда собственная строка удаляемой новости
// в счёт уже не идёт, а если удаление почему-то не состоялось (гонка двух
// вкладок), функция вернёт true и файл останется на месте.
func PhotoUsedByOthers(ctx context.Context, photoURL string) (bool, error) {
	photoURL = strings.TrimSpace(photoURL)
	if photoURL == "" {
		return false, nil
	}

	const query = `SELECT COUNT(*) FROM messages WHERE TRIM(photo_url) = :1`

	var cnt int
	if err := storage.DBSelectOne(ctx, "photo_used_by_others", &cnt, query, photoURL); err != nil {
		return false, err
	}
	return cnt > 0, nil
}
