package web

import (
	// "html/template"
	"bytes"
	// "database/sql"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
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

		newMessage := r.FormValue("new_message")

		var photoURL, photoFIO, photoPost string

		if config.IsHR(pageCtx.DepName) {
			photoFIO = strings.TrimSpace(r.FormValue("photo_fio"))
			photoPost = strings.TrimSpace(r.FormValue("photo_post"))

			// 1. Пытаемся получить файл из формы
			file, header, err := r.FormFile("photo_file")
			if err == nil { // Если файл был прикреплен и ошибки нет
				defer file.Close()

				// 2. Достаем оригинальное расширение (например, ".jpg", ".png")
				ext := filepath.Ext(header.Filename)

				// 3. Формируем безопасное имя файла на основе PHOTO_FIO
				// Заменяем пробелы на подчеркивания, чтобы избежать проблем в URL
				safeFIO := strings.ReplaceAll(photoFIO, " ", "_")

				// Чтобы файлы не перезаписывались, можно добавить отметку времени:
				// fileName := fmt.Sprintf("%s_%d%s", safeFIO, time.Now().Unix(), ext)
				fileName := safeFIO + ext

				// Полный путь для сохранения на сервере
				uploadDir := "static/photos/"
				dstPath := filepath.Join(uploadDir, fileName)

				// 4. Создаем файл на диске сервера
				dst, err := os.Create(dstPath)
				if err != nil {
					slog.Error("Не удалось создать файл на диске", "err", err)
					http.Error(w, "Ошибка сохранения файла", http.StatusInternalServerError)
					return
				}
				defer dst.Close()

				// 5. Копируем содержимое загруженного файла в созданный файл на диске
				if _, err := io.Copy(dst, file); err != nil {
					slog.Error("Ошибка при копировании файла", "err", err)
					http.Error(w, "Ошибка записи файла", http.StatusInternalServerError)
					return
				}

				// В базу данных мы сохраняем ТОЛЬКО имя файла (например, "Ivanov_II.jpg")
				// Префикс "static/photos/" ваш Go-код шага 2 подставит автоматически при чтении!
				photoURL = fileName
			}
		}

		// Вызываем процедуру Oracle, передавая имя файла в качестве URL
		err := storage.DBExec(
			r.Context(),
			"reg.new_message_2",
			pageCtx.FIO,
			pageCtx.DepName,
			newMessage,
			photoURL,
			photoFIO,
			photoPost,
		)

		if err != nil {
			slog.Error("Ошибка выполнения reg.new_message", "err", err)
			http.Error(w, "Ошибка связи с базой данных: "+err.Error(), http.StatusInternalServerError)
			return
		}

		slog.Debug("Функция reg.new_message выполнена")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}
