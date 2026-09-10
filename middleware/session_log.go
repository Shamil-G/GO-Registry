// middleware/session_log.go
//
// Минимальная диагностика уровня Info: «кто зашёл, с какого адреса и что
// менял». Появилась 25.08.2026 после вопроса с боя — в проде не было ВООБЩЕ
// ничего про пользователей.
//
// Как получилось, что диагностика пропала. Раньше `[PCTX] REAL USER` писался
// на Info, то есть на каждый запрос в лог уезжали ФИО, логин, IP, должность и
// подразделение — персональные данные потоком. Строку понизили до Debug,
// а прод работает на INFO (`PROD_LOG_LEVEL=INFO`) — и вместе с шумом исчезла
// и сама возможность ответить «кто у нас сегодня работал». Успешные запросы
// в `middleware/metrics.go` тоже пишутся на Debug (на Warn/Error поднимаются
// только 4xx/5xx), а `Успешная авторизация в SSO` срабатывает лишь на явном
// POST /login, которого у пользователя с живой SSO-сессией просто нет.
//
// Решение — не возвращать поток, а писать две редкие строки:
//
//	[ACCESS] вход в систему   — ОДИН раз на пару «логин + адрес» за TTL,
//	                            а не на каждый запрос;
//	[ACCESS] изменяющий запрос — на POST/PUT/PATCH/DELETE, то есть только там,
//	                            где что-то меняется. GET остаётся на Debug.
//
// Это не заменяет аудит: «что именно изменено» лежит в write-хендлерах и в
// логе PL/SQL-пакетов на стороне Oracle. Здесь ровно тот минимум, которого
// не хватало, чтобы понять, кто вообще был в системе и с какого адреса.
package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// sessionLogTTL — как часто повторять строку входа для одной и той же пары
// «логин + адрес». 8 часов ≈ один рабочий день: пользователь, работающий
// весь день, даёт одну строку, а не тысячу.
const sessionLogTTL = 8 * time.Hour

// sessionsCap — потолок карты, чтобы она не росла бесконечно на длинном
// аптайме. По достижении чистим протухшие записи.
const sessionsCap = 512

var (
	sessionsMu   sync.Mutex
	sessionsSeen = make(map[string]time.Time)
)

// logSessionStart — одна строка Info на сессию, а не на запрос.
func logSessionStart(login, ip string, attrs ...any) {
	if login == "" {
		return
	}

	key := login + "|" + ip
	now := time.Now()

	sessionsMu.Lock()
	first, known := sessionsSeen[key]
	fresh := !known || now.Sub(first) > sessionLogTTL
	if fresh {
		sessionsSeen[key] = now
		if len(sessionsSeen) > sessionsCap {
			for k, t := range sessionsSeen {
				if now.Sub(t) > sessionLogTTL {
					delete(sessionsSeen, k)
				}
			}
		}
	}
	sessionsMu.Unlock()

	if !fresh {
		return
	}

	slog.Info("[ACCESS] вход в систему", append([]any{"user", login, "ip", ip}, attrs...)...)
}

// logMutation — Info на каждый изменяющий запрос: кто, откуда, что дёрнул.
// Исход (403/500) пишет middleware/metrics.go отдельной строкой, здесь важна
// сама привязка «действие → пользователь».
func logMutation(r *http.Request, login, ip string) {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return
	}
	slog.Info("[ACCESS] изменяющий запрос",
		"user", login, "ip", ip, "method", r.Method, "path", r.URL.Path)
}
