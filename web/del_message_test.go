package web

import (
	"html/template"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	message "gusseynov/GO-Registry/service"
)

// TestBackPath — куда возвращается человек после удаления новости. Главное
// здесь не удобство, а то, что чужой адрес из Referer в Location не попадает.
func TestBackPath(t *testing.T) {
	cases := []struct {
		name    string
		referer string
		want    string
	}{
		{"без Referer", "", "/"},
		{"своя страница", "http://registry/time-off", "/time-off"},
		{"своя страница с параметрами", "http://registry/all-list-time-off?flt_month=2026-06", "/all-list-time-off?flt_month=2026-06"},
		// Хост из Referer не используется вовсе — в Location уходит только путь,
		// поэтому чужой сайт превращается в наш же путь, а не в редирект наружу.
		{"чужой сайт", "http://evil.tld/x", "/x"},
		{"протокол-относительный Referer", "//evil.tld/x", "/x"},
		// А вот путь, начинающийся с "//", в Location стал бы
		// протокол-относительным адресом и увёл бы человека на чужой сайт.
		{"путь с двойным слэшем", "http://registry//evil.tld/x", "/"},
		{"мусор", "://", "/"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/del-message", nil)
			if c.referer != "" {
				r.Header.Set("Referer", c.referer)
			}
			if got := backPath(r); got != c.want {
				t.Errorf("backPath(%q) = %q, ожидалось %q", c.referer, got, c.want)
			}
		})
	}
}

// TestSidebarDeleteButton — кнопка удаления рисуется только у своей новости.
// Тест на разметку, потому что именно разметка решает, увидит ли автор кнопку;
// права при этом проверяет DelMessagePost, а не шаблон.
func TestSidebarDeleteButton(t *testing.T) {
	tmpl, err := template.New("sidebar.html").Funcs(template.FuncMap{
		"res_value": func(key string) string { return key },
	}).ParseFiles("../templates/sidebar.html")
	if err != nil {
		t.Fatalf("шаблон ленты не парсится: %v", err)
	}

	data := struct{ AllMessages []message.MessageItem }{
		AllMessages: []message.MessageItem{
			{IDMessage: 11, Message: "своя", CanDelete: true},
			{IDMessage: 22, Message: "чужая", CanDelete: false},
		},
	}

	var buf strings.Builder
	if err := tmpl.ExecuteTemplate(&buf, "info_sidebar", data); err != nil {
		t.Fatalf("рендер ленты: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, `name="id_mess" value="11"`) {
		t.Error("у своей новости нет формы удаления")
	}
	if strings.Contains(out, `name="id_mess" value="22"`) {
		t.Error("у чужой новости появилась форма удаления")
	}
}

// TestRemovePhotoFile — уборка снимка удалённой новости не должна превращаться
// в удаление произвольного файла. Имя файла приходит из photo_url, а туда оно
// попадает из поля формы photo_fio без всякой проверки (пункт 0.5 плана), то
// есть управляется тем же человеком, который жмёт "удалить".
func TestRemovePhotoFile(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "outside.html")
	photos := filepath.Join(dir, "photos")
	if err := os.MkdirAll(photos, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(path string) {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("обычная картинка удаляется", func(t *testing.T) {
		p := filepath.Join(photos, "Ivanov_I_I.jpg")
		write(p)
		if err := removePhotoFile(photos, "Ivanov_I_I.jpg"); err != nil {
			t.Fatalf("removePhotoFile: %v", err)
		}
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Error("файл остался на диске")
		}
	})

	t.Run("выход за каталог не срабатывает", func(t *testing.T) {
		write(outside)
		// Base оставит "outside.html", расширение не картиночное — файл цел.
		if err := removePhotoFile(photos, "../outside.html"); err != nil {
			t.Fatalf("removePhotoFile: %v", err)
		}
		if _, err := os.Stat(outside); err != nil {
			t.Errorf("файл за пределами каталога снимков удалён: %v", err)
		}
	})

	t.Run("traversal с картиночным расширением", func(t *testing.T) {
		target := filepath.Join(dir, "victim.png")
		write(target)
		if err := removePhotoFile(photos, "../victim.png"); err != nil {
			t.Fatalf("removePhotoFile: %v", err)
		}
		if _, err := os.Stat(target); err != nil {
			t.Errorf("удалён файл вне каталога снимков: %v", err)
		}
	})

	t.Run("обратные слэши", func(t *testing.T) {
		target := filepath.Join(dir, "win.png")
		write(target)
		if err := removePhotoFile(photos, `..\win.png`); err != nil {
			t.Fatalf("removePhotoFile: %v", err)
		}
		if _, err := os.Stat(target); err != nil {
			t.Errorf("удалён файл вне каталога снимков: %v", err)
		}
	})

	t.Run("не картинка остаётся", func(t *testing.T) {
		p := filepath.Join(photos, "base.html")
		write(p)
		if err := removePhotoFile(photos, "base.html"); err != nil {
			t.Fatalf("removePhotoFile: %v", err)
		}
		if _, err := os.Stat(p); err != nil {
			t.Error("удалён файл, не похожий на картинку")
		}
	})

	t.Run("пустое имя и отсутствующий файл — не ошибка", func(t *testing.T) {
		if err := removePhotoFile(photos, "   "); err != nil {
			t.Errorf("пустое имя: %v", err)
		}
		if err := removePhotoFile(photos, "nothing.png"); err != nil {
			t.Errorf("отсутствующий файл: %v", err)
		}
	})
}
