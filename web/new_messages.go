package web

import (
	// "html/template"
	"bytes"
	// "database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"gusseynov/GO-Registry/config"
	"gusseynov/GO-Registry/middleware"
	message "gusseynov/GO-Registry/service"
	"gusseynov/GO-Registry/service/i18n"
	"gusseynov/GO-Registry/storage"
)

type ViewMessage struct {
	// 💡 Встраиваем базовый контекст (подтянет UserName, DepName, Lang, Theme)
	*middleware.BasePageContext
	Message     string
	IsHR        bool
	ListTimeOff []TimeOffItem
	AllMessages []message.MessageItem
}

func NewMessageGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Извлекаем уже готовый и посчитанный контекст из нашей мидлвари Authorize
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		// data := ViewTimeOff{
		// 	BasePageContext: pageCtx,
		// 	Message:         "",
		// }

		data := ViewMessage{
			BasePageContext: pageCtx,
			IsHR:            config.IsHR(pageCtx.DepName),
			Message:         "",
		}
		// 4. Компиляция шаблонов с привязкой i18n
		tmpl, err := template.New("base.html").Funcs(template.FuncMap{
			"res_value": func(key string) string {
				return i18n.Get(pageCtx.Lang, key)
			},
		}).ParseFiles(
			"templates/base.html",
			"templates/new_message.html",
			"templates/sidebar.html", // Вынесенная инфо-панель
		)

		if err != nil {
			slog.Error("Ошибка компиляции шаблона new_message", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Безопасный рендеринг через буфер
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
			slog.Error("Ошибка выполнения шаблона модуля отсутствий", "err", err)
			http.Error(w, "Ошибка рендеринга", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = buf.WriteTo(w)

	}
}

// AllListReportHandler генерирует Excel отчет и отправляет его пользователю на скачивание
func NewMessagePost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		// Потолок на тело запроса ставится ДО первого чтения формы: r.FormValue
		// разбирает multipart сам, и без этой строки гигабайтный файл уехал бы
		// во временный каталог ещё до того, как мы посмотрим на него.
		limitPhotoBody(w, r)

		if err := r.ParseMultipartForm(maxPhotoBytes); err != nil {
			// Сюда же приходит превышение лимита из MaxBytesReader.
			slog.Warn("[NewMessage] форма не разобрана (возможно, слишком большой файл)",
				"user", pageCtx.LoginName, "err", err)
			http.Error(w, i18n.Get(pageCtx.Lang, "ERR_PHOTO_TOO_BIG"), http.StatusRequestEntityTooLarge)
			return
		}

		newMessage := r.FormValue("new_message")

		var photoURL, photoFIO, photoPost string

		if config.IsHR(pageCtx.DepName) {
			photoFIO = strings.TrimSpace(r.FormValue("photo_fio"))
			photoPost = strings.TrimSpace(r.FormValue("photo_post"))

			// Файл необязателен: новость может быть и без фотографии.
			file, header, err := r.FormFile("photo_file")
			if err == nil {
				defer file.Close()

				// Имя файла, расширение и Content-Type из формы НЕ используются
				// вовсе — всё решает содержимое, см. web/photo_upload.go.
				// header нужен только для лога.
				name, err := savePhoto(photoUploadDir, file)
				switch {
				case err == errPhotoType:
					slog.Warn("[NewMessage] отклонён файл: содержимое не похоже на картинку",
						"user", pageCtx.LoginName, "filename", header.Filename, "size", header.Size)
					http.Error(w, i18n.Get(pageCtx.Lang, "ERR_PHOTO_TYPE"), http.StatusBadRequest)
					return
				case err != nil:
					slog.Error("[NewMessage] не удалось сохранить фото",
						"user", pageCtx.LoginName, "filename", header.Filename, "err", err)
					http.Error(w, i18n.Get(pageCtx.Lang, "ERR_PHOTO_SAVE"), http.StatusInternalServerError)
					return
				}

				slog.Info("[NewMessage] фото принято",
					"stored", name, "orig", header.Filename, "size", header.Size, "user", pageCtx.LoginName)

				// В БД пишем ТОЛЬКО имя файла: префикс static/photos/
				// подставляется при чтении ленты (service.GetAllMessage).
				photoURL = name
			}
		}

		// Вызываем процедуру Oracle, передавая имя файла в качестве URL
		err := storage.DBExec(
			r.Context(),
			"reg.new_message",
			pageCtx.FIO,
			pageCtx.DepName,
			newMessage,
			photoURL,
			photoFIO,
			photoPost,
		)

		if err != nil {
			slog.Error("Ошибка выполнения reg.new_message", "user", pageCtx.LoginName, "err", err)
			http.Error(w, i18n.Get(pageCtx.Lang, "ERR_DB"), http.StatusInternalServerError)
			return
		}

		slog.Debug("Функция reg.new_message выполнена")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
