// web/same_origin.go
//
// Защита изменяющих запросов от CSRF. Перенос из GO-SRM/GO-Court 25.08.2026.
//
// Почему привычная защита кукой здесь не работает. Личность определяется по
// IP-сессии SSO (sso.CheckSession по адресу клиента), а не по нашей сессионной
// куке — значит SameSite на куках lang/theme не мешает вообще ничему:
// посторонняя страница, открытая сотрудником, может отправить форму на
// /approve-time-off, запрос уйдёт с его адреса и выполнится с его правами.
// Токен в куке (double-submit) по той же причине половинчат, а для внутренней
// сети без выхода в интернет ещё и избыточен.
//
// Поэтому сверяем, что запрос пришёл с нашей же страницы. Браузеры на POST
// всегда присылают Origin (и почти всегда Referer), и подделать их из JS
// нельзя — это заголовки под контролем самого браузера.
//
// ВАЖНО про прокси (грабли GO-SRM, 25.08.2026). Наивное сравнение с r.Host
// работает только при прямом обращении. Если перед сервисом стоит обратный
// прокси, подставляющий в Host адрес апстрима (proxy_pass без
// proxy_set_header Host), то r.Host — внутренний адрес, Origin — внешний, и
// КАЖДЫЙ POST, включая /login, получает 403. Поэтому адрес портала берётся из
// трёх источников:
//
//  1. X-Forwarded-Host — если прокси его проставляет (nginx: proxy_set_header
//     X-Forwarded-Host $http_host). Из браузера подделать нельзя: не входит в
//     CORS-safelist, кросс-сайтовая форма его не поставит, а fetch с ним ушёл
//     бы в preflight, на который портал не отвечает.
//  2. r.Host — прямое обращение и правильно настроенный прокси.
//  3. PUBLIC_HOSTS из .env — явный список внешних адресов портала, когда
//     прокси не наш и трогать его нельзя. Формат:
//     "registry.gfss.kz,192.168.1.34:9001" (схема и хвостовой слэш, если
//     попадут, отбрасываются).
//
// Не-браузерные клиенты (curl, скрипты, мониторинг) заголовков не шлют — их
// пропускаем, иначе сломаем отладку и служебные вызовы.
//
// Это НЕ замена проверкам прав в хендлерах — она отвечает только на вопрос
// «пришло ли это с нашей страницы», а не «можно ли этому пользователю».
package web

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"gusseynov/GO-Registry/config"
	mdw "gusseynov/GO-Registry/middleware"
)

// RequireSameOrigin — пропускает только те изменяющие запросы, которые пришли
// с нашей же страницы. GET/HEAD не трогает: они не меняют состояние.
func RequireSameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodPut &&
			r.Method != http.MethodPatch && r.Method != http.MethodDelete {
			next.ServeHTTP(w, r)
			return
		}

		// Origin — основной источник правды. Referer — запасной: часть
		// корпоративных прокси и антивирусов Origin режет.
		source := r.Header.Get("Origin")
		if source == "" {
			source = r.Header.Get("Referer")
		}
		if source == "" {
			// Ни того, ни другого — это не браузерная форма. Пропускаем, но
			// оставляем след: если такие запросы пойдут потоком, это видно.
			slog.Debug("[SameOrigin] запрос без Origin и Referer", "path", r.URL.Path, "method", r.Method)
			next.ServeHTTP(w, r)
			return
		}

		u, err := url.Parse(source)
		if err != nil || u.Host == "" {
			page := mdw.GetOrCreatePageCtx(r.Context())
			slog.Warn("[SameOrigin] неразбираемый источник запроса",
				"source", source, "path", r.URL.Path, "user", page.LoginName, "ip", page.IP)
			http.Error(w, "Запрос отклонён: не удалось определить источник формы", http.StatusForbidden)
			return
		}

		allowed := allowedHosts(r)
		if hostAllowed(u.Host, allowed) {
			next.ServeHTTP(w, r)
			return
		}

		page := mdw.GetOrCreatePageCtx(r.Context())
		slog.Warn("[SameOrigin] запрос с чужого сайта отклонён",
			"source", source,
			"source_host", u.Host,
			"our_host", r.Host,
			"x_forwarded_host", r.Header.Get("X-Forwarded-Host"),
			"allowed", strings.Join(allowed, ","),
			"path", r.URL.Path,
			"user", page.LoginName, "ip", page.IP)
		http.Error(w, "Запрос отклонён: форма отправлена не со страницы портала", http.StatusForbidden)
	})
}

// allowedHosts — адреса, по которым портал может быть открыт в браузере.
// Порядок важен только для читаемости лога; сравнение идёт по всему списку.
func allowedHosts(r *http.Request) []string {
	hosts := make([]string, 0, 4)

	if h := forwardedHost(r); h != "" {
		hosts = append(hosts, h)
	}
	if r.Host != "" {
		hosts = append(hosts, r.Host)
	}
	if config.Cfg != nil {
		hosts = append(hosts, config.Cfg.PublicHosts...)
	}
	return hosts
}

// forwardedHost — первый элемент X-Forwarded-Host. Заголовок может быть
// списком ("внешний, промежуточный"), первым идёт адрес, который набрал
// пользователь.
func forwardedHost(r *http.Request) string {
	xfh := r.Header.Get("X-Forwarded-Host")
	if xfh == "" {
		return ""
	}
	if i := strings.IndexByte(xfh, ','); i >= 0 {
		xfh = xfh[:i]
	}
	return strings.TrimSpace(xfh)
}

// hostAllowed — есть ли host в списке (регистронезависимо, с нормализацией
// значений из .env: там легко написать "http://registry.gfss.kz/").
func hostAllowed(host string, allowed []string) bool {
	for _, a := range allowed {
		if a == "" {
			continue
		}
		if strings.EqualFold(host, normalizeHost(a)) {
			return true
		}
	}
	return false
}

// normalizeHost — оставляет от значения только "хост:порт": срезает схему,
// путь и хвостовой слэш. Нужно для PUBLIC_HOSTS, где формат задаёт человек.
func normalizeHost(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, "://"); i >= 0 {
		v = v[i+3:]
	}
	if i := strings.IndexByte(v, '/'); i >= 0 {
		v = v[:i]
	}
	return v
}
