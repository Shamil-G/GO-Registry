// web/photo_upload.go
//
// Приём фотографии для новостной ленты (пункт 0.5 плана, 10.09.2026).
//
// Что было до этой правки в web/new_messages.go:
//
//	ext := filepath.Ext(header.Filename)                 // ".html", ".svg" — любое
//	safeFIO := strings.ReplaceAll(photoFIO, " ", "_")    // менялись ТОЛЬКО пробелы
//	os.Create(filepath.Join(uploadDir, safeFIO+ext))
//
// Отсюда четыре разные беды, и каждая лечится ниже:
//
//  1. ПУТЬ. Имя собиралось из поля формы photo_fio, точки и слэши в нём никто
//     не трогал. photo_fio = "../../templates/base" плюс файл x.html — и
//     приложение перезаписывает собственный шаблон. Теперь имя генерируем сами
//     и из формы не берём ни байта.
//  2. ТИП. Расширение бралось из имени загруженного файла, а static/photos/
//     раздаётся http.FileServer с того же origin, что и портал. .svg и .html
//     браузер выполняет — то есть хранимый XSS в сессии любого, кто открыл
//     ленту (а сессия здесь привязана к IP). Теперь тип определяется по
//     содержимому файла, а не по его имени, и разрешены только растровые
//     картинки.
//  3. РАЗМЕР. Лимита не было вовсе. Теперь тело запроса обёрнуто в
//     MaxBytesReader.
//  4. КАТАЛОГ. os.Create без MkdirAll: пропал static/photos/ — 500-я на
//     ровном месте.
//
// Имя файла теперь уникальное (время + случайный хвост), а не «ФИО с
// подчёркиваниями». Побочно чинится ещё одна тихая беда: два человека с
// одинаковым ФИО на карточке затирали снимок друг друга, и у старой новости
// незаметно менялась фотография.
package web

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// maxPhotoBytes — потолок на всё тело запроса с новостью, включая текст.
// 5 МБ: фотография сотрудника для панели шириной ~400 пикселей столько не
// весит и близко, а память сервера при этом не зависит от того, что прислали.
const maxPhotoBytes = 5 << 20

// photoTypes — что мы готовы положить в static/photos/ и раздавать наружу.
//
// Ключ — то, что вернул http.DetectContentType по содержимому файла, значение —
// расширение, под которым файл ляжет на диск. Именно в эту сторону, а не
// наоборот: расширение назначаем мы по факту содержимого, имя из формы не
// участвует.
//
// SVG в списке нет и быть не должно: это XML со скриптами внутри, браузер
// исполнит его как страницу с нашего origin. Ровно этой дырой и был старый код.
var photoTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
	"image/bmp":  ".bmp",
}

// errPhotoType — содержимое файла не похоже на картинку из списка выше.
var errPhotoType = fmt.Errorf("недопустимый тип файла")

// limitPhotoBody — обёртка тела запроса, после которой ParseMultipartForm
// вернёт ошибку на слишком большом файле, а не съест память.
// Вызывать ДО первого r.FormValue: тот разбирает форму сам.
func limitPhotoBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPhotoBytes)
}

// savePhoto кладёт присланный файл в dir под сгенерированным именем и
// возвращает это имя (в БД пишется только оно, префикс static/photos/
// подставляется при чтении ленты).
//
// Возвращает errPhotoType, если содержимое не похоже на картинку.
func savePhoto(dir string, file multipart.File) (string, error) {
	// Тип — по первым 512 байтам самого файла. http.DetectContentType
	// смотрит сигнатуры, поэтому переименованный в .jpg html сюда не пройдёт.
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return "", err
	}
	head = head[:n]

	ext, ok := photoTypes[stripCharset(http.DetectContentType(head))]
	if !ok {
		return "", errPhotoType
	}

	// Отматываем назад: 512 байт уже прочитаны, без Seek они не попадут в файл.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	name, err := photoFileName(ext)
	if err != nil {
		return "", err
	}

	dst, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return "", err
	}
	defer dst.Close()

	// io.Copy, а не ReadAll: файл не обязан целиком помещаться в память.
	// Тело запроса уже ограничено MaxBytesReader, второй лимит не нужен.
	if _, err := io.Copy(dst, file); err != nil {
		// Недописанный файл на диске не нужен — он всё равно битый.
		_ = os.Remove(filepath.Join(dir, name))
		return "", err
	}

	return name, nil
}

// photoFileName — уникальное имя вида "1757490000_9f3c1a72.jpg".
//
// Время впереди, чтобы файлы в каталоге сортировались по дате и глазами было
// видно, что старое; случайный хвост — чтобы две загрузки в одну секунду не
// столкнулись. Из данных пользователя в имени нет ничего.
func photoFileName(ext string) (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d_%s%s", time.Now().Unix(), hex.EncodeToString(buf), ext), nil
}

// stripCharset — "text/html; charset=utf-8" → "text/html".
func stripCharset(ct string) string {
	for i := 0; i < len(ct); i++ {
		if ct[i] == ';' {
			return ct[:i]
		}
	}
	return ct
}
