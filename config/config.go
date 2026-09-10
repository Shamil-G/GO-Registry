package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

var PhoneBook string

// Config хранит все настройки нашего портала
type Config struct {
	IsProd     bool
	LogLevel   string
	ListenAddr string
	SSOServer  string
	// TrustedServers []string
	LOGIN_PAGE  string
	PUBLIC_PATH []string
	// PublicHosts — внешние адреса портала, какими их видит браузер, из
	// PUBLIC_HOSTS в .env. Нужны web.RequireSameOrigin, когда перед сервисом
	// стоит обратный прокси, переписывающий Host на адрес апстрима и не
	// проставляющий X-Forwarded-Host. Пустой список — норма при прямом
	// обращении.
	PublicHosts   []string
	Boss          []string
	ApproveAdmins []string
	HRList        []string
	// MetricsAllowedIPs — кому отдавать /metrics (METRICS_ALLOWED_IPS).
	// Пустой список означает «всем», как было раньше; при старте про это
	// пишется предупреждение. Подробности — web/metrics_access.go.
	MetricsAllowedIPs []string
	// SecurityList — департаменты службы безопасности (SECURITY_DEPARTMENT).
	// СБ ведёт собственный список отсутствий (/secure-time-off) и следит, кто и
	// когда приходит на работу. Пустой список — норма и безопасный дефолт: тогда
	// раздел остаётся только у супер-администраторов, как было в меню.
	SecurityList []string
	//
	TESTER_LOGIN_NAME string
	TESTER_TOP_LEVEL  int
	TESTER_TOP_VIEW   int
	// Oracle DB
	DBServer             string
	DBUser               string
	DBPassword           string
	DBServiceName        string
	DBMinConns           string
	DBMaxConns           string
	DBPoolInc            string
	DBExpireTinme        string // = 15  # количество минут между отправкой keepalive
	DBTimeout            string // = 300     # В секундах. Время простоя, после которого курсор освобождается
	DBWaitTime           string // = 2000  # Время (в миллисекундах) ожидания доступного сеанса в пуле, перед тем как выдать ошибку
	DBMaxLifeTimeSession string // = 180  # Время в секундах, в течении которого может существоват сеанс
}

var Cfg *Config

// Вспомогательная функция для задания дефолтных значений
func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func ChooseAddr(IsProduction bool) string {
	server_port := os.Getenv("PROD_PORT")
	server_addr := getEnv("PROD_SERVER_ADDR", "192.168.1.34")

	if !IsProduction {
		server_addr = getEnv("DEV_SERVER_ADDR", "127.0.0.1")
		server_port = os.Getenv("DEVELOP_PORT")
		slog.Info("Detected DEVELOP mode (Windows)")
	} else {
		slog.Info("Detected PRODUCTION mode (Linux)")
	}

	return server_addr + ":" + server_port
}

func IsPublicPath(path string) bool {
	for _, p := range Cfg.PUBLIC_PATH {
		// 1. Корневой путь "/" — только точное совпадение
		if p == "/" {
			if path == "/" {
				return true
			}
			continue
		}

		// 2. Префиксные пути ("/static/", "/language/", "/theme/")
		if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(path, p) {
				return true
			}
			continue
		}

		// 3. Точное совпадение ("/login", "/bd")
		if path == p {
			return true
		}
	}
	return false
}

func parseCSVList(raw string) []string {
	result := make([]string, 0)
	parts := strings.Split(raw, ",")

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// LoadConfig читает файл .env и возвращает заполненную структуру
func LoadConfig(IsProduction bool) error {
	// Загружаем переменные из файла .env в окружение процесса
	// Если файла нет (например, на проде переменные заданы через Docker), godotenv не упадет
	_ = godotenv.Load()

	listenAddr := ChooseAddr(IsProduction)

	dbMaxConns := "4"
	sso_server := getEnv("DEV_SSO_SERVER", "")

	if IsProduction {
		dbMaxConns = getEnv("DB_MAX_CONNS", "8")
		sso_server = getEnv("PROD_SSO_SERVER", "")
	}

	// Извлекаем строку доверенных серверов и бьем её по запятой в массив
	PhoneBook = os.Getenv("PHONE_BOOK")
	topLevel, _ := strconv.Atoi(getEnv("TESTER_TOP_LEVEL", "0"))
	topView, _ := strconv.Atoi(getEnv("TESTER_TOP_VIEW", "0"))
	Cfg = &Config{
		IsProd:        IsProduction,
		ListenAddr:    listenAddr,
		SSOServer:     sso_server,
		LOGIN_PAGE:    getEnv("LOGIN_PAGE", "/login"),
		Boss:          parseCSVList(os.Getenv("Boss")),
		ApproveAdmins: parseCSVList(os.Getenv("ApproveAdmins")),
		HRList:        parseCSVList(os.Getenv("HR_DEPARTMENT")),
		SecurityList:  parseCSVList(os.Getenv("SECURITY_DEPARTMENT")),

		MetricsAllowedIPs: parseCSVList(os.Getenv("METRICS_ALLOWED_IPS")),
		PUBLIC_PATH:       parseCSVList(os.Getenv("PUBLIC_PATHS")),
		PublicHosts:       parseCSVList(os.Getenv("PUBLIC_HOSTS")),
		//
		TESTER_LOGIN_NAME: getEnv("TESTER_LOGIN_NAME", "/login"),
		TESTER_TOP_LEVEL:  topLevel,
		TESTER_TOP_VIEW:   topView,
		// Oracle
		DBServer:      os.Getenv("DB_SERVER"),
		DBUser:        os.Getenv("DB_USER"),
		DBPassword:    os.Getenv("DB_PASSWORD"),
		DBServiceName: os.Getenv("DB_SERVICE"),
		DBMaxConns:    dbMaxConns,
		DBPoolInc:     os.Getenv("DBPoolInc"),
		DBExpireTinme: getEnv("DBExpireTinme", "15"), // = 15  # количество минут между отправкой keepalive
		DBTimeout:     getEnv("DBTimeout", "300"),    // = 300     # В секундах. Время простоя, после которого курсор освобождается
		DBWaitTime:    getEnv("DBWaitTime", "2000"),  // = 2000  # Время (в миллисекундах) ожидания доступного сеанса в пуле, перед тем как выдать ошибку
		// = 180  # Время в секундах, в течении которого может существоват сеанс
		DBMaxLifeTimeSession: getEnv("DBMaxLifeTimeSession", "180"),
	}
	// LoadPublicPaths()
	LoadTrustedProxies()
	slog.Info("[LoadConfig]", "PUBLIC_PATH", Cfg.PUBLIC_PATH, "PUBLIC_HOSTS", Cfg.PublicHosts)
	slog.Debug("[LoadConfig]", "Boss", Cfg.Boss, "ApproveAdmins", Cfg.ApproveAdmins)

	// Предупреждение, а не молчание: /metrics отдаёт карту сервиса и имена
	// процедур Oracle, и открытым он остаётся только по недосмотру.
	if len(Cfg.MetricsAllowedIPs) == 0 {
		slog.Warn("METRICS_ALLOWED_IPS не задан — /metrics доступен всем, кто может открыть портал")
	} else {
		slog.Info("METRICS_ALLOWED_IPS загружен", "ips", Cfg.MetricsAllowedIPs)
	}
	return nil
}
