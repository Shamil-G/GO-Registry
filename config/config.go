package config

import (
	"log/slog"
	"os"
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
	local_port := os.Getenv("DEVELOP_PORT")
	remote_port := os.Getenv("PROD_PORT")
	server_addr := getEnv("SERVER_ADDR", "192.168.1.34")

	if !IsProduction {
		slog.Info("Detected DEVELOP mode (Windows)")
		return "127.0.0.1:" + local_port
	}

	slog.Info("Detected PRODUCTION mode (Linux)")
	return server_addr + ":" + remote_port
}

func LoadPublicPaths() {
	raw := os.Getenv("PUBLIC_PATHS")
	Cfg.PUBLIC_PATH = make([]string, 0) // важно!

	if raw == "" {
		return
	}

	parts := strings.Split(raw, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			Cfg.PUBLIC_PATH = append(Cfg.PUBLIC_PATH, p)
		}
	}
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

// LoadConfig читает файл .env и возвращает заполненную структуру
func LoadConfig(IsProduction bool) error {
	// Загружаем переменные из файла .env в окружение процесса
	// Если файла нет (например, на проде переменные заданы через Docker), godotenv не упадет
	_ = godotenv.Load()

	listenAddr := ChooseAddr(IsProduction)

	dbMaxConns := "4"

	if IsProduction {
		dbMaxConns = getEnv("DB_MAX_CONNS", "8")
	}

	// Извлекаем строку доверенных серверов и бьем её по запятой в массив
	PhoneBook = os.Getenv("PHONE_BOOK")

	Cfg = &Config{
		IsProd:     IsProduction,
		ListenAddr: listenAddr,
		SSOServer:  getEnv("SSO_SERVER", "192.168.1.34:8025"),
		LOGIN_PAGE: getEnv("LOGIN_PAGE", "/login"),
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
	LoadPublicPaths()
	slog.Info("[LoadConfig]", "PUBLIC_PATH", Cfg.PUBLIC_PATH)
	return nil
}
