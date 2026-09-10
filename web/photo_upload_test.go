package web

import (
	"bytes"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pngBytes — минимальный настоящий PNG (сигнатура + IHDR), достаточный для
// http.DetectContentType.
var pngBytes = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89,
}

// asMultipart отдаёт содержимое как multipart.File — ровно то, что приходит из
// r.FormFile. Имя файла в форме задаётся отдельно, чтобы проверить: оно на
// результат не влияет вообще.
func asMultipart(t *testing.T, filename string, content []byte) multipart.File {
	t.Helper()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("photo_file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	mr := multipart.NewReader(&buf, mw.Boundary())
	form, err := mr.ReadForm(int64(len(content)) + 1024)
	if err != nil {
		t.Fatal(err)
	}
	f, err := form.File["photo_file"][0].Open()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// TestSavePhotoAcceptsImage — картинка сохраняется, имя генерируем мы, и
// содержимое доезжает до диска целиком (первые 512 байт читались для
// определения типа — без Seek они бы потерялись).
func TestSavePhotoAcceptsImage(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "photos") // каталога нет — savePhoto создаёт сам

	name, err := savePhoto(dir, asMultipart(t, "фото.png", pngBytes))
	if err != nil {
		t.Fatalf("savePhoto: %v", err)
	}
	if filepath.Ext(name) != ".png" {
		t.Errorf("расширение назначено не по содержимому: %q", name)
	}
	if strings.Contains(name, "фото") {
		t.Errorf("имя из формы попало в имя файла: %q", name)
	}

	got, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("файл не создан: %v", err)
	}
	if !bytes.Equal(got, pngBytes) {
		t.Errorf("содержимое файла испорчено: получено %d байт из %d", len(got), len(pngBytes))
	}
}

// TestSavePhotoRejectsNonImage — главная проверка этого файла. static/photos/
// раздаётся с того же origin, что и портал, поэтому .svg и .html здесь
// означают выполнение чужого скрипта в сессии любого, кто открыл ленту.
func TestSavePhotoRejectsNonImage(t *testing.T) {
	cases := []struct {
		name     string
		filename string
		content  string
	}{
		{"html под видом jpg", "photo.jpg", "<html><script>alert(1)</script></html>"},
		{"svg со скриптом", "photo.svg", `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`},
		{"обычный текст", "photo.png", "просто текст, а не картинка"},
		{"пустой файл", "photo.png", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()

			if _, err := savePhoto(dir, asMultipart(t, c.filename, []byte(c.content))); err != errPhotoType {
				t.Fatalf("ожидалась errPhotoType, получено: %v", err)
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 0 {
				t.Errorf("отклонённый файл всё-таки попал на диск: %v", entries)
			}
		})
	}
}

// TestPhotoFileNameUnique — имена не должны сталкиваться: раньше файл назывался
// по ФИО с карточки, и вторая загрузка молча затирала снимок в уже
// опубликованной новости.
func TestPhotoFileNameUnique(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		name, err := photoFileName(".jpg")
		if err != nil {
			t.Fatal(err)
		}
		if seen[name] {
			t.Fatalf("повтор имени файла: %s", name)
		}
		seen[name] = true

		// Имя должно быть пригодно как последний элемент пути и никуда не уводить.
		if filepath.Base(name) != name {
			t.Fatalf("имя содержит разделители пути: %q", name)
		}
	}
}
