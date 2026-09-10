// web/del_message.go
//
// Удаление новости из ленты автором (10.09.2026).
//
// Правило простое: убрать новость может тот, кто её опубликовал, — и никто
// больше. Ни руководитель, ни HR, ни супер-администратор: список
// ApproveAdmins про согласование отсутствий, а не про чужие объявления.
// Понадобится исключение для админа — это одна строка ниже, но заводить его
// без спроса не стоит.
//
// Три уровня, и все три обязательны:
//
//  1. Кнопка в ленте рисуется только у своих новостей
//     (MessageItem.CanDelete, см. service/messages.go). Это удобство.
//  2. Здесь автор ДОЧИТЫВАЕТСЯ ИЗ БД по id и сверяется с page.FIO. id лежит
//     в скрытом поле формы — подменить его на чужой можно за минуту через
//     инспектор браузера, поэтому разметка правами не распоряжается.
//  3. Сама reg.del_message удаляет строку только при совпадении автора
//     (см. db/reg_del_message.sql). Дублирование сознательное: если завтра
//     процедуру позовут из другого места, забыв проверку, ничего не сломается.
//
// От подделки формы с чужого сайта защищает web.RequireSameOrigin на группе
// роутов — сессия здесь привязана к IP, поэтому запрос с рабочей станции
// сотрудника в остальном неотличим от его собственного.
package web

import (
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gusseynov/GO-Registry/middleware"
	message "gusseynov/GO-Registry/service"
	"gusseynov/GO-Registry/service/i18n"
	"gusseynov/GO-Registry/storage"
)

// DelMessagePost — POST /del-message.
func DelMessagePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())
		lang := pageCtx.Lang

		if pageCtx.IsAnonymous {
			slog.Warn("[DelMessage] анонимная попытка удаления новости", "ip", pageCtx.IP)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		idMess, err := strconv.Atoi(strings.TrimSpace(r.FormValue("id_mess")))
		if err != nil || idMess <= 0 {
			slog.Warn("[DelMessage] некорректный идентификатор новости",
				"id_mess", r.FormValue("id_mess"), "user", pageCtx.LoginName)
			http.Error(w, i18n.Get(lang, "ERR_BAD_ID"), http.StatusBadRequest)
			return
		}

		// Кто автор на самом деле — по данным БД, а не по тому, что прислала форма.
		// Заодно забираем имя файла фотографии: после удаления записи прочитать
		// его будет уже негде.
		author, photoURL, found, err := message.MessageOwner(r.Context(), idMess)
		if err != nil {
			slog.Error("[DelMessage] не удалось прочитать автора новости",
				"id_mess", idMess, "user", pageCtx.LoginName, "err", err)
			http.Error(w, i18n.Get(lang, "ERR_DB"), http.StatusInternalServerError)
			return
		}
		if !found {
			// Обычный случай: открытая со вчера вкладка, новость уже удалена.
			// Info, а не Warn — это не попытка обойти проверку.
			slog.Info("[DelMessage] новость не найдена (устаревшая страница)",
				"id_mess", idMess, "user", pageCtx.LoginName)
			http.Error(w, i18n.Get(lang, "ERR_MESSAGE_NOT_FOUND"), http.StatusNotFound)
			return
		}

		if !strings.EqualFold(author, strings.TrimSpace(pageCtx.FIO)) {
			// Warn: событие аудита. Кнопки на чужой новости нет, значит форму
			// правили руками — это должно быть видно в проде без переключения
			// уровня логирования.
			slog.Warn("[DelMessage] попытка удалить чужую новость",
				"id_mess", idMess, "author", author,
				"user", pageCtx.LoginName, "user_fio", pageCtx.FIO, "ip", pageCtx.IP)
			http.Error(w, i18n.Get(lang, "ERR_NOT_MESSAGE_AUTHOR"), http.StatusForbidden)
			return
		}

		// Автор передаётся в процедуру и проверяется там ещё раз — см. шапку файла.
		// IP последним параметром: в messages не видно, с какого адреса
		// заводили и убирали новость, а в логе пакета это остаётся.
		if err := storage.DBExec(r.Context(), "reg.del_message", idMess, pageCtx.FIO, pageCtx.IP); err != nil {
			slog.Error("[DelMessage] ошибка выполнения reg.del_message",
				"id_mess", idMess, "user", pageCtx.LoginName, "err", err)
			http.Error(w, i18n.Get(lang, "ERR_DB"), http.StatusInternalServerError)
			return
		}

		slog.Info("[DelMessage] новость удалена автором",
			"id_mess", idMess, "user", pageCtx.LoginName, "ip", pageCtx.IP)

		// Фотография — уже после удаления записи и только если на неё никто
		// больше не ссылается. Ошибки здесь не показываем человеку: новость
		// удалена, задача выполнена, а осиротевший файл — вопрос уборки, а не
		// отказа. Всё, что нужно, остаётся в логе.
		cleanupPhoto(r, photoURL, idMess, pageCtx.LoginName)

		http.Redirect(w, r, backPath(r), http.StatusSeeOther)
	}
}

// backPath — страница, на которую вернуть человека после удаления. Лента
// висит сбоку на половине страниц портала, и возвращать всех на "/" неудобно.
//
// Из Referer берётся ТОЛЬКО путь: подставить в Location чужой адрес целиком —
// это открытый редирект, каким бы доверенным ни выглядел источник. Пустой,
// внешний ("//evil.tld") или неразбираемый Referer превращается в "/".
func backPath(r *http.Request) string {
	ref := r.Header.Get("Referer")
	if ref == "" {
		return "/"
	}

	u, err := url.Parse(ref)
	if err != nil || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") {
		return "/"
	}

	back := u.Path
	if u.RawQuery != "" {
		back += "?" + u.RawQuery
	}
	return back
}

// photoUploadDir — каталог, куда NewMessagePost кладёт снимки для ленты.
// Держится здесь же, где удаление, чтобы запись и уборка не разъехались.
const photoUploadDir = "static/photos"

// allowedPhotoExt — расширения, файлы с которыми удаление считает своими.
// Это НЕ дубль валидации загрузки (её пока нет — пункт 0.5 плана, из-за него
// в static/photos/ может лежать что угодно), а страховка удаления: убирать с
// диска мы готовы только картинку.
var allowedPhotoExt = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true, ".bmp": true,
}

// cleanupPhoto — удалить снимок удалённой новости, если он больше никому не нужен.
func cleanupPhoto(r *http.Request, photoURL string, idMess int, user string) {
	if strings.TrimSpace(photoURL) == "" {
		return
	}

	used, err := message.PhotoUsedByOthers(r.Context(), photoURL)
	if err != nil {
		slog.Error("[DelMessage] не удалось проверить, используется ли фото",
			"photo", photoURL, "id_mess", idMess, "err", err)
		return
	}
	if used {
		// Штатный случай: имя файла собирается из ФИО на снимке, поэтому одна
		// карточка переиспользуется в нескольких новостях про одного человека.
		slog.Info("[DelMessage] фото оставлено: на него ссылаются другие новости",
			"photo", photoURL, "id_mess", idMess)
		return
	}

	if err := removePhotoFile(photoUploadDir, photoURL); err != nil {
		slog.Error("[DelMessage] не удалось удалить фото новости",
			"photo", photoURL, "id_mess", idMess, "user", user, "err", err)
		return
	}
	slog.Info("[DelMessage] фото новости удалено", "photo", photoURL, "id_mess", idMess, "user", user)
}

// removePhotoFile — удаление файла из каталога снимков.
//
// ПОЧЕМУ ЗДЕСЬ СТОЛЬКО ПРОВЕРОК ДЛЯ ОДНОГО os.Remove. Имя файла собирается из
// поля формы photo_fio и кладётся в БД как есть (web/new_messages.go, пункт
// 0.5 плана — валидации загрузки там до сих пор нет). Значит в photo_url
// может лежать "../../templates/base.html", и наивное
// os.Remove(filepath.Join(dir, name)) превратило бы удаление своей новости в
// удаление любого файла приложения: опубликовал — удалил — снёс шаблон.
// Поэтому от имени берётся только последний элемент пути, результат
// проверяется на выход за пределы каталога, а расширение — по белому списку.
//
// dir параметром, а не через константу, ради тестов: проверять такое на живом
// static/photos/ нельзя.
func removePhotoFile(dir, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	// Только последний элемент пути: "../../x.png" → "x.png". ToSlash сначала —
	// в БД имя могло попасть и с обратными слэшами (Windows-клиент).
	base := filepath.Base(filepath.FromSlash(strings.ReplaceAll(name, "\\", "/")))
	if base == "." || base == ".." || base == string(filepath.Separator) {
		slog.Warn("[DelMessage] подозрительное имя файла фото, удаление пропущено", "photo", name)
		return nil
	}

	if !allowedPhotoExt[strings.ToLower(filepath.Ext(base))] {
		slog.Warn("[DelMessage] файл фото не похож на картинку, удаление пропущено",
			"photo", name, "ext", filepath.Ext(base))
		return nil
	}

	// Пояс и подтяжки: даже после Base сверяем, что итоговый путь остался
	// внутри каталога снимков.
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	path := filepath.Join(absDir, base)
	if filepath.Dir(path) != absDir {
		slog.Warn("[DelMessage] путь фото выходит за каталог снимков, удаление пропущено",
			"photo", name, "path", path)
		return nil
	}

	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			// Файла нет — его уже убрали руками или новость публиковали без
			// снимка при заполненном photo_url. Не ошибка.
			slog.Info("[DelMessage] файла фото на диске нет", "photo", base)
			return nil
		}
		return err
	}
	return nil
}
