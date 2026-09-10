package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// capture — перехват записей slog, чтобы считать строки, а не глазами читать лог.
type capture struct {
	mu   sync.Mutex
	msgs []string
}

func (c *capture) Enabled(context.Context, slog.Level) bool { return true }

func (c *capture) Handle(_ context.Context, r slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msgs = append(c.msgs, r.Message)
	return nil
}

func (c *capture) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *capture) WithGroup(string) slog.Handler      { return c }

func (c *capture) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.msgs)
}

func withCapture(t *testing.T) *capture {
	t.Helper()
	c := &capture{}
	prev := slog.Default()
	slog.SetDefault(slog.New(c))
	t.Cleanup(func() { slog.SetDefault(prev) })

	sessionsMu.Lock()
	sessionsSeen = make(map[string]time.Time)
	sessionsMu.Unlock()

	return c
}

// Смысл всей затеи: строка на СЕССИЮ, а не на запрос. Иначе вернётся тот же
// поток персональных данных, из-за которого её когда-то и понизили до Debug.
func TestLogSessionStart(t *testing.T) {
	c := withCapture(t)

	for i := 0; i < 5; i++ {
		logSessionStart("Гусейнов", "192.168.5.59", "dep", "ДИТиТО")
	}
	if got := c.count(); got != 1 {
		t.Fatalf("повторные запросы дали %d строк, ожидалась 1", got)
	}

	logSessionStart("Гусейнов", "192.168.5.60") // тот же логин, другой адрес
	if got := c.count(); got != 2 {
		t.Fatalf("смена адреса не дала новой строки: строк %d", got)
	}

	logSessionStart("", "192.168.5.61") // аноним — писать нечего
	if got := c.count(); got != 2 {
		t.Fatalf("аноним попал в лог: строк %d", got)
	}
}

// Протухание: после TTL пользователь снова считается «зашедшим».
func TestLogSessionStartПовторПослеTTL(t *testing.T) {
	c := withCapture(t)

	logSessionStart("Гусейнов", "192.168.5.59")

	sessionsMu.Lock()
	sessionsSeen["Гусейнов|192.168.5.59"] = time.Now().Add(-sessionLogTTL - time.Minute)
	sessionsMu.Unlock()

	logSessionStart("Гусейнов", "192.168.5.59")
	if got := c.count(); got != 2 {
		t.Fatalf("после TTL строк %d, ожидалось 2", got)
	}
}

// GET в Info не пишем — иначе получится сплошной access-log.
func TestLogMutation(t *testing.T) {
	c := withCapture(t)

	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		logMutation(httptest.NewRequest(m, "/risks/17", nil), "Гусейнов", "192.168.5.59")
	}
	if got := c.count(); got != 0 {
		t.Fatalf("читающие запросы попали в Info: строк %d", got)
	}

	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		logMutation(httptest.NewRequest(m, "/risks/17/fill", nil), "Гусейнов", "192.168.5.59")
	}
	if got := c.count(); got != 4 {
		t.Fatalf("изменяющих строк %d, ожидалось 4", got)
	}
}
