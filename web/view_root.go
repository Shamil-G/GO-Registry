// handlers/web/view_root.go

package web

import (
	"bytes"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"gusseynov/GO-Registry/config"
	"gusseynov/GO-Registry/middleware"
	message "gusseynov/GO-Registry/service"
	"gusseynov/GO-Registry/service/i18n"
	ssoPkg "gusseynov/GO-Registry/sso"
)

// BirthdayTarget — структура для записей дней рождения (оставляем)
type BirthdayTarget struct {
	BirthDate string
	Employee  string
	Post      string
	DepName   string
}

// MessageItem — структура для сообщений (оставляем)
type MessageItem struct {
	IDMessage int
	Date      string
	Author    string
	DepName   string
	Message   string
}

// ViewData — основная структура данных для шаблона
type ViewData struct {
	*middleware.BasePageContext // 💡 Встраиваем базовый контекст (UserName, DepName, Lang, Theme, IsBoss, IsAnonymous)
	HasSubordinates             bool
	IsAdmin                     bool
	IsBoss                      bool // Дублируем для обратной совместимости, если в шаблоне используется .IsBoss, а не встроенный .BasePageContext.IsBoss
	UserPost                    string
	UserDep                     string
	ListBD                      []ssoPkg.BirthdayUser // Используем тип из ssoPkg для точного маппинга

	// 💡 Используем строго типизированную структуру из пакета сервиса!
	AllMessages []message.MessageItem
	// Для скачик PhoneBook
	PhoneBook string
}

// ViewRootGet отображает главную страницу сайта (GET /)
func ViewRootGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Сквозное чтение объявлений — теперь работает железно для всех, отдавая [] при пустоте
		messages := message.GetAllMessage(r.Context())

		startSSO := time.Now()
		// 3. Сквозное чтение именинников из SSO — тоже доступно анонимам
		birthdays, err := ssoPkg.Client.GetBirthdays(r.Context())
		if err != nil {
			slog.Error("[ROOT] Ошибка получения списка именинников", "err", err, "duration", time.Since(startSSO))
			birthdays = []ssoPkg.BirthdayUser{} // Защита от nil
		} else {
			// slog.Debug("[ROOT] Именинники успешно получены", "duration", time.Since(startSSO))
		}

		// 1. Извлекаем готовый контекст, собранный нашей мидлварью Authorize.
		// Теперь она гарантированно пропускает и гостей (IsAnonymous=true), и шефов.
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		var data ViewData
		// 🔐 БЕЗОПАСНЫЙ ЩИТ: Заходим в объект User только если это НЕ аноним!
		// && !pageCtx.IsAnonymous
		if !pageCtx.IsAnonymous {
			// Высчитываем специфичные роли руководителя на основе данных SSO профиля
			isAdmin := config.IsSuperAdmin(pageCtx.FIO)
			hasSubordinates := len(pageCtx.SubordinateOU) > 0

			slog.Debug("[ROOT]", "LoginName", pageCtx.LoginName, "FIO", pageCtx.FIO)

			// Заполняем поля структуры для авторизованного отображения
			data = ViewData{
				BasePageContext: pageCtx,
				ListBD:          birthdays,
				AllMessages:     messages,
				IsAdmin:         isAdmin,
				HasSubordinates: hasSubordinates,
				PhoneBook:       config.PhoneBook,
			}
		} else {
			anonCtx := &middleware.BasePageContext{
				IsAnonymous: true,
				Lang:        pageCtx.Lang,
				Theme:       pageCtx.Theme,
			}
			data = ViewData{
				BasePageContext: anonCtx,
				ListBD:          birthdays,
				AllMessages:     messages,
			}
		}

		// 5. Компиляция комплекта шаблонов с динамической привязкой i18n
		tmpl, err := template.New("base.html").Funcs(template.FuncMap{
			"add": func(a, b int) int { return a + b },
			"res_value": func(key string) string {
				return i18n.Get(data.Lang, key) // Язык берется прямо из нашего контекста мидлвари
			},
		}).ParseFiles(
			"templates/base.html",
			"templates/home.html",
		)

		if err != nil {
			slog.Error("Ошибка компиляции пары шаблонов base+home", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// 6. Безопасный рендеринг через буфер с точным указанием корневой точки "base.html"
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base.html", data); err != nil {
			slog.Error("[ROOT] Ошибка выполнения шаблона главной страницы", "err", err)
			http.Error(w, "Ошибка рендеринга: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = buf.WriteTo(w)
	}
}

func ViewRootPost() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	}
}
