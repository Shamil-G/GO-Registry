// web/require.go
//
// Проверки прав, навешиваемые на роут (пункт 0.4 плана, 10.09.2026).
//
// Почему мидлварь на роуте, а не три строки в начале хендлера. Потому что три
// строки в начале хендлера забывают. В этом проекте так и вышло: пункты меню в
// home.html спрятаны за {{ if .IsAdmin }} / {{ if .IsHR }}, а сами адреса
// открыты любому авторизованному — прятать кнопку и закрывать страницу это
// разные вещи. У /secure-time-off роутов два (GET и POST), и «щит» стоял
// только на одном; при таком раскладе новый роут раздела получает проверку
// автоматически, а не «если не забудут скопировать».
//
// Проверки ставятся ПОСЛЕ mdw.PageContext: без него в контексте пусто и
// закрылось бы для всех.
package web

import (
	"log/slog"
	"net/http"

	"gusseynov/GO-Registry/config"
	mdw "gusseynov/GO-Registry/middleware"
	"gusseynov/GO-Registry/service/i18n"
)

// RequireHR — раздел новостной ленты: публикация объявлений на главной.
// Форма и так рисуется только для HR, но POST до сих пор принимал кого угодно.
func RequireHR(next http.Handler) http.Handler {
	return requireRole(next, "HR", func(p *mdw.BasePageContext) bool {
		return p.IsAdmin || config.IsHR(p.DepName)
	})
}

// RequireSecurity — раздел службы безопасности: свой список отсутствий
// (status = 3) и контроль прихода на работу. Департаменты СБ задаются
// SECURITY_DEPARTMENT в .env; пока там пусто, раздел доступен только
// супер-администраторам — как он и выглядел в меню.
func RequireSecurity(next http.Handler) http.Handler {
	return requireRole(next, "Security", func(p *mdw.BasePageContext) bool {
		return p.IsAdmin || config.IsSecurity(p.DepName)
	})
}

// RequireAdmin — только супер-администраторы из ApproveAdmins.
func RequireAdmin(next http.Handler) http.Handler {
	return requireRole(next, "Admin", func(p *mdw.BasePageContext) bool {
		return p.IsAdmin
	})
}

// requireRole — общая часть: аноним отправляется на логин, остальным при
// отказе отдаётся 403 с нейтральным текстом.
func requireRole(next http.Handler, role string, allowed func(*mdw.BasePageContext) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := mdw.GetOrCreatePageCtx(r.Context())

		if page.IsAnonymous {
			http.Redirect(w, r, config.Cfg.LOGIN_PAGE, http.StatusSeeOther)
			return
		}

		if !allowed(page) {
			// Info, а не Warn: чаще всего это сохранённая ссылка или адрес,
			// подсмотренный у коллеги, а не попытка взлома. Но в логе должно
			// быть видно, если раздел закрыт не тому кругу людей.
			slog.Info("[Access] раздел закрыт по роли",
				"need", role, "path", r.URL.Path, "method", r.Method,
				"user", page.LoginName, "dep", page.DepName, "ip", page.IP)
			http.Error(w, i18n.Get(page.Lang, "ERR_NO_RIGHTS"), http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
