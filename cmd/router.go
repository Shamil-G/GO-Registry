// cmd/router.go

package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	mdw "gusseynov/GO-Registry/middleware"
	ssoPkg "gusseynov/GO-Registry/sso"
	webH "gusseynov/GO-Registry/web"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func Router() http.Handler {
	r := chi.NewRouter()

	// Мидлвары
	// r.Use(mdw.Authorize)
	// r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// r.Use(mdw.SlogLogger)
	r.Use(mdw.Metrics)

	r.Handle("/metrics", promhttp.Handler())
	// Health
	r.Post("/ping", ssoPkg.Alive())
	// Статика
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Настройки пользователя
	// -----------------------------
	// WEB UI (внутренний интерфейс)
	// -----------------------------
	r.Group(func(r chi.Router) {
		r.Use(mdw.PageContext)
		// Главная
		// Без этого не сработает middleware и не будет получен контекст
		// зарегистрированного пользователя
		r.Get("/", webH.ViewRootGet())

		r.Get("/language/{lang}", webH.ChangeLangHandler())
		r.Get("/theme/{theme}", webH.ChangeThemeHandler())

		// Авторизация
		r.Get("/login", webH.LoginFormGet())
		r.Post("/login", webH.LoginPost())
		// web.Post("/login", sso.LoginPost())
		r.Get("/logout", webH.LogoutGet())

		// регистрация заявок
		r.Post("/time-off", webH.TimeOffPost())
		r.Get("/time-off", webH.TimeOffGet())
		r.Post("/del-from-list-time-off", webH.DelFromListTimeOff())

		// утверждение заявок
		r.Get("/list-to-approve", webH.ListToApproveGet())
		r.Post("/refuse-time-off", webH.RefuseTimeOffPost())
		r.Post("/approve-time-off", webH.ApproveTimeOffPost())

		//
		r.Get("/list-npa", webH.ListNPA())
		r.Post("/log-click", webH.LogClick())
		r.Post("/uploads-use-npa", webH.UploadsUseNpa())

		// Список, который заполняют сами безопасники
		r.Post("/secure-time-off", webH.SecureTimeOffPost())
		r.Get("/secure-time-off", webH.SecureTimeOffGet())

		// Отчет по списку
		r.Get("/all-list-time-off", webH.AllListTimeOff())
		r.Post("/all-list-time-off", webH.AllListTimeOff())
		r.Get("/all_list_report", webH.AllListReport())

		r.Get("/list-absent", webH.ListAbsent())

		r.Get("/new-message", webH.NewMessageGet())
		r.Post("/new-message", webH.NewMessagePost())
	})

	return r
}
