package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gusseynov/GO-Registry/config"
)

// Сценарий, который положил GO-SRM 25.08.2026: прокси переписал Host на адрес
// апстрима, браузер прислал внешний Origin.
func TestHostAllowed(t *testing.T) {
	saved := config.Cfg
	t.Cleanup(func() { config.Cfg = saved })

	// Значения намеренно «грязные»: в .env легко написать со схемой и слэшем.
	config.Cfg = &config.Config{PublicHosts: []string{"http://registry.gfss.kz/", " 192.168.1.34:9001 "}}

	r := httptest.NewRequest(http.MethodPost, "/approve-time-off", strings.NewReader("id=1"))
	r.Host = "127.0.0.1:9001" // внутренний адрес, подставленный прокси

	cases := map[string]bool{
		"registry.gfss.kz":      true,
		"REGISTRY.GFSS.KZ":      true,
		"192.168.1.34:9001":     true,
		"127.0.0.1:9001":        true,
		"evil.example":          false,
		"registry.gfss.kz:8080": false, // порт значим
	}
	for host, want := range cases {
		if got := hostAllowed(host, allowedHosts(r)); got != want {
			t.Errorf("hostAllowed(%q) = %v, ожидалось %v", host, got, want)
		}
	}

	r.Header.Set("X-Forwarded-Host", "portal.gfss.kz:8443, inner")
	if !hostAllowed("portal.gfss.kz:8443", allowedHosts(r)) {
		t.Error("X-Forwarded-Host не учтён")
	}
}

// Поведение middleware целиком: своя форма проходит, чужая — 403, GET и
// запросы без заголовков не трогаем.
func TestRequireSameOrigin(t *testing.T) {
	saved := config.Cfg
	t.Cleanup(func() { config.Cfg = saved })
	config.Cfg = &config.Config{}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	cases := []struct {
		name, method, origin, xfh string
		wantStatus                int
	}{
		{"POST со своей страницы", http.MethodPost, "http://registry.gfss.kz", "", http.StatusOK},
		{"POST с чужого сайта", http.MethodPost, "http://evil.example", "", http.StatusForbidden},
		{"POST за прокси: Host внутренний, X-Forwarded-Host свой", http.MethodPost, "http://portal.gfss.kz", "portal.gfss.kz", http.StatusOK},
		{"POST без заголовков (curl, мониторинг)", http.MethodPost, "", "", http.StatusOK},
		{"GET с чужим Origin не трогаем", http.MethodGet, "http://evil.example", "", http.StatusOK},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(c.method, "/approve-time-off", strings.NewReader("id=1"))
			r.Host = "registry.gfss.kz"
			if c.name == "POST за прокси: Host внутренний, X-Forwarded-Host свой" {
				r.Host = "127.0.0.1:9001"
			}
			if c.origin != "" {
				r.Header.Set("Origin", c.origin)
			}
			if c.xfh != "" {
				r.Header.Set("X-Forwarded-Host", c.xfh)
			}
			rec := httptest.NewRecorder()
			RequireSameOrigin(next).ServeHTTP(rec, r)
			if rec.Code != c.wantStatus {
				t.Errorf("статус %d, ожидали %d (тело: %s)", rec.Code, c.wantStatus, rec.Body.String())
			}
		})
	}
}
