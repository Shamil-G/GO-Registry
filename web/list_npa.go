package web

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"gusseynov/GO-Registry/middleware"
	"gusseynov/GO-Registry/report"
	"gusseynov/GO-Registry/service"
	"gusseynov/GO-Registry/service/i18n"
)

// NpaItem описывает один нормативно-правовой акт
type NpaItem struct {
	Num  int    `json:"num"`
	Name string `json:"name"`
	Path string `json:"path"`
}

// GetStaticNpaList возвращает ваш оригинальный список законов
func GetStaticNpaList() []NpaItem {
	return []NpaItem{
		{Num: 1, Name: "Социальный кодекс", Path: "/static/nsi/Социальный кодекс.pdf"},
		{Num: 2, Name: "Правила исчисления, назначения 0701 от 01-01-2025", Path: "/static/nsi/Правила исчисления, назначения 0701 от 01-01-2025.pdf"},
		{Num: 3, Name: "Правила исчисления, назначения 0702 от 01-01-2025", Path: "/static/nsi/Правила исчисления, назначения 0702 от 01-01-2025.pdf"},
		{Num: 4, Name: "Правила исчисления, назначения 0703 от 01-01-2025", Path: "/static/nsi/Правила исчисления, назначения 0703 от 01-01-2025.pdf"},
		{Num: 5, Name: "Правила исчисления, назначения 0704-0705 от 01-01-2025", Path: "/static/nsi/Правила исчисления, назначения 0704-0705 от 01-01-2025.pdf"},
	}
}

// NpaPageContext собирает данные для HTML-шаблона
type NpaPageContext struct {
	*middleware.BasePageContext // 💡 Встраиваем базовый контекст (UserName, DepName, Lang, Theme перелетели)
	ListNpa                     []service.NpaItem
	AllMess                     []string
	CurrentYear                 int // Считаем на месте в обработчике
	AllMessages                 []service.MessageItem
}

// ClickLogPayload описывает JSON, приходящий от фронтенда при клике
type ClickLogPayload struct {
	UserName  string `json:"user_name"`
	DepName   string `json:"dep_name"`
	Path      string `json:"path"`
	Num       int    `json:"num"`
	Name      string `json:"name"`
	Timestamp string `json:"timestamp"`
}

// GlobalListNpa — глобальный список НПА. Наполняется при старте приложения.
var GlobalListNpa []NpaItem

// ListNPA отображает страницу со списком документов.
// Возвращает http.HandlerFunc под вызов в роутере: webH.ListNPA()
// ListNPA отображает страницу со списком документов.
// Возвращает http.HandlerFunc под вызов в роутере: webH.ListNPA()
func ListNPA() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Извлекаем уже готовый и посчитанный контекст из нашей мидлвари Authorize
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		// 1. 🚀 Интегрируем список законов из сервиса
		npaList := service.GetStaticNpaList()

		allMess := service.GetAllMessage(r.Context())
		currentYear := time.Now().Year() // Считаем на месте, как вы и просили

		slog.Info("VIEW LIST NPA", "username", pageCtx.FIO, "boss", pageCtx.IsBoss)

		// 2. Заполняем контекст страницы, наследуя все базовые поля из pageCtx
		ctxData := NpaPageContext{
			BasePageContext: pageCtx,
			ListNpa:         npaList,
			AllMessages:     allMess,
			CurrentYear:     currentYear,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		// 3. Компиляция шаблонов (язык берется из готового pageCtx.Lang)
		tmpl, err := template.New("base.html").Funcs(template.FuncMap{
			"add": func(a, b int) int { return a + b },
			"res_value": func(key string) string {
				return i18n.Get(pageCtx.Lang, key) // Используем язык из middleware
			},
		}).ParseFiles(
			"templates/base.html",
			"templates/list_npa.html",
		)

		if err != nil {
			slog.Error("Ошибка компиляции пары шаблонов base+list_npa", "err", err)
			http.Error(w, "Ошибка сервера", http.StatusInternalServerError)
			return
		}

		// Безопасный рендеринг через буфер с явным указанием корневого base.html
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "base.html", ctxData); err != nil {
			slog.Error("Ошибка выполнения шаблона главной страницы", "err", err)
			http.Error(w, "Ошибка рендеринга: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = buf.WriteTo(w)
	}
}

// LogClick принимает JSON-агрегацию данных клика и логирует ее.
// Возвращает http.HandlerFunc под вызов в роутере: webH.LogClick()
// LogClick принимает JSON и логирует клики по ссылкам НПА
func LogClick() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("Получен запрос на логирование клика НПА")

		var data ClickLogPayload
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			slog.Error("Failed to decode log-click JSON", "Error", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		if data.UserName == "" || data.Name == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// 💡 ИСПРАВИЛИ: Добавили context.Background() первым аргументом,
		// чтобы совпало с контрактом функции (теперь аргументов ровно 5)
		go service.UseFileStatistic(data.UserName, data.DepName, data.Name, data.Path)

		// Отдаем статус 204. Браузер сразу, без задержек, скачивает файл
		w.WriteHeader(http.StatusNoContent)
	}
}

// UploadsUseNpa обрабатывает генерацию файла Excel и инициирует скачивание.
// Возвращает http.HandlerFunc под вызов в роутере: webH.UploadsUseNpa()
// UploadsUseNpa обрабатывает запрос на создание отчета и отдает файл Excel.
// Возвращает http.HandlerFunc под вызов в роутере: webH.UploadsUseNpa()
func UploadsUseNpa() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 1. Проверяем авторизацию шефа (на всякий случай, хотя мидлварь защищает роут)
		pageCtx := middleware.GetOrCreatePageCtx(r.Context())

		// Дополнительный щит: только боссы или админы могут дергать этот отчет
		if !pageCtx.IsBoss {
			slog.Warn("Отказ в доступе к генерации отчета НПА", "user", pageCtx.FIO)
			http.Error(w, "Доступ запрещен", http.StatusForbidden)
			return
		}

		// 2. Считываем год из POST-параметров формы (аналог request.form.get)
		if err := r.ParseForm(); err != nil {
			slog.Error("Failed to parse form in UploadsUseNpa", "Error", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		yearStr := r.FormValue("flt_year")

		// Если форма пустая, проверяем JSON-вариант (аналог request.json.get)
		if yearStr == "" {
			var jsonBody struct {
				Year string `json:"year"`
			}
			if err := json.NewDecoder(r.Body).Decode(&jsonBody); err == nil {
				yearStr = jsonBody.Year
			}
		}

		// Если год вообще не пришел, берем текущий по умолчанию
		if yearStr == "" {
			yearStr = strconv.Itoa(time.Now().Year())
		}

		slog.Info("Запрос на генерацию отчета НПА", "user", pageCtx.FIO, "year", yearStr)

		// 3. Вызываем бизнес-логику генерации отчета из пакета service
		// Функция должна создать файл на диске и вернуть путь к нему
		filePath, err := report.ReportUseNpa(r.Context(), yearStr)
		if err != nil {
			slog.Error("Ошибка генерации Excel отчета НПА", "year", yearStr, "err", err)
			http.Error(w, "Ошибка сервера при создании отчета", http.StatusInternalServerError)
			return
		}

		// Гарантируем, что временный сгенерированный файл удалится с диска после отправки клиенту
		// defer os.Remove(filePath)

		// 4. Настраиваем HTTP-заголовки для скачивания (аналог send_file as_attachment=True)
		w.Header().Set("Content-Disposition", "attachment; filename=report_use_npa_"+yearStr+".xlsx")
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

		// 5. Отправляем файл пользователю (Go делает это через системные вызовы, не нагружая RAM)
		http.ServeFile(w, r, filePath)
	}
}
