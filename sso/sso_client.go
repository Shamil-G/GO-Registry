// sso/client.go
package sso

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"gusseynov/GO-Registry/config"
	"gusseynov/GO-Registry/metrics"
)

type SSOClient struct {
	httpClient *http.Client
}

// BirthdayUser — структура для парсинга именинников (остается без изменений)
type BirthdayUser struct {
	BirthDate     string `json:"birth_date"`
	BirthDateSort string `json:"birth_date_sort"`
	DepName       string `json:"dep_name"`
	Employee      string `json:"employee"`
	Post          string `json:"post"`
}

// SSOUser — НОВАЯ полная структура пользователя из актуального ответа SSO
type SSOUser struct {
	LoginName     string   `json:"login_name"`
	FIO           string   `json:"fio"`
	Post          string   `json:"post"`
	RfbnID        string   `json:"rfbn_id"`
	DepName       string   `json:"dep_name"`
	Time          string   `json:"time"`      // Для /check
	LastTime      string   `json:"last_time"` // Для /login
	Lang          string   `json:"lang"`      // Вы добавите на стороне SSO
	Theme         string   `json:"theme"`     // Вы добавите на стороне SSO
	DN            string   `json:"dn"`
	PrincipalName string   `json:"principalName"`
	OUName        string   `json:"ou_name"`
	LegacyName    string   `json:"legacy_name"`
	SubordinateOU []string `json:"subordinate_ou"`
}

// SSOResponse — корень ответа для /check и /login
type SSOResponse struct {
	Status int     `json:"status"`
	User   SSOUser `json:"user"`
}

// SSOResponse — корень ответа для /check и /login
type SSOSet struct {
	Status int     `json:"status"`
	User   SSOUser `json:"user"`
}

// Глобальная переменная для SSO
var Client *SSOClient

func Init() {
	Client = &SSOClient{
		httpClient: &http.Client{
			// Раз у нас запросы летят быстрее 1мс, 500мс — идеальный таймаут с запасом
			// Timeout: 500 * time.Millisecond,
		},
	}
}

// Alive — ручка /ping
func Alive() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	}
}

func (s *SSOClient) doSSORequest(endpoint string, req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := s.httpClient.Do(req)

	elapsed := time.Since(start).Microseconds()

	code := "error"
	if err == nil {
		code = http.StatusText(resp.StatusCode)
	}

	metrics.SSORequestDuration.WithLabelValues(endpoint, code).Observe(float64(elapsed))

	metrics.SSORequestTotal.WithLabelValues(endpoint, code).Inc()

	return resp, err
}

// GetBirthdays — без изменений
func (s *SSOClient) GetBirthdays(ctx context.Context) ([]BirthdayUser, error) {
	url := "http://" + config.Cfg.SSOServer + "/bd"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// resp, err := s.httpClient.Do(req)
	resp, err := s.doSSORequest("bd", req)
	if err != nil {
		slog.Warn("SSO BD", "SSO /bd недоступен", err)
		return nil, fmt.Errorf("SSO /bd недоступен: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SSO /bd вернул статус: %d", resp.StatusCode)
	}

	var list []BirthdayUser
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON именинников: %w", err)
	}

	return list, nil
}

// CheckSession — ОБНОВЛЕН под POST-запрос с JSON и новый ответ
// CheckSession — ОБНОВЛЕН под POST-запрос с JSON и безопасный ручной разбор плавающих типов
// sso/client.go

// 💡 ВСЕЯДНЫЙ МЕТОД LOGIN: парсит сырую мапу и защищает от "" строк
func (s *SSOClient) Login(ctx context.Context, login, password, ip string) (*SSOUser, error) {
	payload := map[string]string{
		"login_name": login,
		"password":   password,
		"ip_addr":    ip,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := "http://" + config.Cfg.SSOServer + "/login"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// resp, err := s.httpClient.Do(req)
	resp, err := s.doSSORequest("login", req)

	if err != nil {
		slog.Warn("SSO LOGIN", "SSO /login недоступен", err)
		return nil, fmt.Errorf("SSO /login недоступен: %w", err)
	}
	defer resp.Body.Close()

	// 1. Распаковываем в универсальную мапу map[string]any
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("ошибка декодирования JSON ответа /login: %w", err)
	}

	// Проверяем числовой статус ответа (в JSON числа всегда float64)
	statusFloat, _ := raw["status"].(float64)
	if int(statusFloat) != http.StatusOK {
		return nil, fmt.Errorf("ошибка авторизации, статус от SSO: %d", int(statusFloat))
	}

	// 2. Извлекаем вложенный объект "user" из ответа SSO
	rawUser, ok := raw["user"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("объект user не найден в ответе SSO")
	}

	// 3. Собираем структуру пользователя вручную через хелпер parseSSOUser
	user := parseSSOUser(rawUser)

	return user, nil
}

// 💡 ВСЕЯДНЫЙ МЕТОД CHECKSESSION: теперь тоже защищен ручным разбором через switch!
func (s *SSOClient) CheckSession(ctx context.Context, clientIP string) (*SSOUser, error) {
	url := "http://" + config.Cfg.SSOServer + "/check"

	payload := map[string]string{
		"login_name": "",
		"ip_addr":    clientIP,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// resp, err := s.httpClient.Do(req)
	resp, err := s.doSSORequest("check", req)
	if err != nil {
		slog.Warn("SSO CHECK", "SSO /check недоступен", err)
		return nil, fmt.Errorf("SSO /check недоступен: %w", err)
	}
	defer resp.Body.Close()

	// 1. Распаковываем ответ в универсальную мапу map[string]any вместо старого Decode(&ssoResp)
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		slog.Warn("SSO CHECK", "ошибка декодирования JSON ответа /check", err)
		return nil, fmt.Errorf("ошибка декодирования JSON ответа /check: %w", err)
	}

	// Проверяем статус ответа
	statusFloat, _ := raw["status"].(float64)
	if int(statusFloat) != http.StatusOK {
		return nil, fmt.Errorf("сессия не активна (статус SSO: %d)", int(statusFloat))
	}

	// 2. Извлекаем вложенный объект "user"
	rawUser, ok := raw["user"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("объект user не найден в ответе SSO /check")
	}

	// 3. Собираем структуру пользователя через общий хелпер parseSSOUser
	user := parseSSOUser(rawUser)

	return user, nil
}

// ----------------------------------------------------------------------
// Вспомогательные внутренние хелперы для ручного разбора (Dry Code)
// ----------------------------------------------------------------------

// parseSSOUser вручную собирает структуру SSOUser и безопасно обрабатывает subordinate_ou
func parseSSOUser(rawUser map[string]any) *SSOUser {
	getStr := func(m map[string]any, key string) string {
		if val, ok := m[key].(string); ok {
			return val
		}
		return ""
	}

	user := &SSOUser{
		FIO:       getStr(rawUser, "fio"),
		LoginName: getStr(rawUser, "login_name"),
		Post:      getStr(rawUser, "post"),
		DepName:   getStr(rawUser, "dep_name"),
		Theme:     getStr(rawUser, "theme"),
		Lang:      getStr(rawUser, "lang"),
	}

	// НАШ ЧИТАЕМЫЙ РУЧНОЙ ЩИТ ДЛЯ СТРОК И МАССИВОВ
	switch v := rawUser["subordinate_ou"].(type) {
	case string:
		user.SubordinateOU = []string{}
		if v != "" && v != " " {
			user.SubordinateOU = []string{v}
		}
	case []any:
		user.SubordinateOU = make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				user.SubordinateOU = append(user.SubordinateOU, str)
			}
		}
	default:
		user.SubordinateOU = []string{}
	}

	return user
}

// CloseSession — остается, метод актуален
func (s *SSOClient) CloseSession(ctx context.Context, ip string) error {
	payload := map[string]string{"ip_addr": ip}
	body, _ := json.Marshal(payload)

	url := "http://" + config.Cfg.SSOServer + "/close"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	// resp, err := s.httpClient.Do(req)
	resp, err := s.doSSORequest("close", req)

	if err != nil {
		slog.Warn("SSO CLose", "SSO /close недоступен", err)
		return fmt.Errorf("SSO /close недоступен: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// Предположим, у вас структура называется Client, и у неё есть поле Address (URL сервера SSO)
// и httpClient. Если имена другие — подправьте под ваш код.
func (s *SSOClient) Set(ctx context.Context, ip string, field string, value string) error {
	// Собираем карту строго под ваш рабочий контракт
	requestBody := map[string]string{
		"ip_addr": ip,    // <-- ИСПРАВИЛИ КЛЮЧ НА ip_addr
		field:     value, // Сюда динамически запишется "lang": "kz" или "theme": "dark"
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		slog.Error("SSO SET", "Ошибка маршалинга запроса /set", err)
		return fmt.Errorf("ошибка маршалинга запроса /set: %w", err)
	}

	url := "http://" + config.Cfg.SSOServer + "/set"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		slog.Error("SSO SET", "Ошибка создания HTTP-запроса /set", err)
		return fmt.Errorf("ошибка создания HTTP-запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// resp, err := c.httpClient.Do(req)
	resp, err := s.doSSORequest("set", req)

	if err != nil {
		slog.Error("SSO SET", "Сервер SSO недоступен /set", err)
		return fmt.Errorf("сервер SSO недоступен: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("SSO SET", "Сервис вернула статус", err)
		return fmt.Errorf("SSO ручка /set вернула статус: %d", resp.StatusCode)
	}

	return nil
}
