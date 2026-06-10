package storage

import (
	// "database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"gusseynov/GO-Registry/config"

	"github.com/jmoiron/sqlx"
	_ "github.com/sijms/go-ora/v2"
)

// DB — глобальный пул подключений, к которому мы будем обращаться
var DB *sqlx.DB

// Init ищет все настройки прямо в config.Cfg и поднимает пул
func Init() error {
	// Собираем строку подключения без таскания параметров
	connStr := "oracle://" + config.Cfg.DBUser + ":" + config.Cfg.DBPassword + "@" + config.Cfg.DBServer + "/" + config.Cfg.DBServiceName
	slog.Info("INIT DB", "CONNECTION", connStr)

	var err error
	DB, err = sqlx.Open("oracle", connStr)
	if err != nil {
		slog.Error("INIT DB", "Ошибка открытия пула Oracle", err)
		return fmt.Errorf("ошибка открытия пула Oracle: %w", err)
	}

	// Парсим максимальное количество соединений из строки в число
	maxConns, err := strconv.Atoi(config.Cfg.DBMaxConns)
	if err != nil {
		slog.Warn("Не удалось распознать DB_MAX_CONNS, ставим дефолт 4", "err", err)
		maxConns = 4
	}

	// Настраиваем лимиты пула
	DB.SetMaxOpenConns(maxConns)
	DB.SetMaxIdleConns(maxConns / 2)
	if maxConns <= 2 {
		DB.SetMaxIdleConns(1)
	}

	// Задаем время жизни сессии (можно также вытащить таймауты из config.Cfg, если нужно)
	DB.SetConnMaxLifetime(5 * time.Minute)

	// Проверяем реальный отклик от базы
	if err = DB.Ping(); err != nil {
		slog.Error("INIT DB", "База Oracle недоступна (ping failed)", err)
		return fmt.Errorf("База Oracle недоступна (ping failed): %w", err)
	}

	return nil
}
