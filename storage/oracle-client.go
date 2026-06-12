// storage/oracle-client.go

package storage

import (
	// "database/sql"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"gusseynov/GO-Registry/config"
	"gusseynov/GO-Registry/metrics"

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
	slog.Info("[INIT DB] DB Opened")

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
	slog.Info("[INIT DB] База Oracle доступна (ping succeeded)")

	// 3. Запускаем метрики пула
	metrics.StartDBPoolMetrics(DB.DB)
	return nil
}

// placeholders(3) → ":1,:2,:3"
func placeholders(n int) string {
	if n == 0 {
		return ""
	}
	s := ""
	for i := 1; i <= n; i++ {
		if i > 1 {
			s += ","
		}
		s += ":" + strconv.Itoa(i)
	}
	return s
}

// CallSPContext — вызов хранимой процедуры Oracle с метриками
func DBExec(ctx context.Context, procName string, args ...any) error {
	start := time.Now()

	call := "BEGIN " + procName + "(" + placeholders(len(args)) + "); END;"

	_, err := DB.ExecContext(ctx, call, args...)
	elapsed := time.Since(start).Milliseconds()

	metrics.DBSPDuration.WithLabelValues(procName).Observe(float64(elapsed))
	metrics.DBSPTotal.WithLabelValues(procName).Inc()

	if err != nil {
		metrics.DBSPErrors.WithLabelValues(procName, err.Error()).Inc()
		slog.Error("[DBExec]", "Критическая ошибка выполнения", procName, "parameters", placeholders(len(args)), "err", err)
	}

	return err
}

// CallSPContext — вызов хранимой процедуры Oracle с метриками
func DBExecQuery(ctx context.Context, procName string, args ...any) (*sql.Rows, error) {
	start := time.Now()

	call := "BEGIN " + procName + "(" + placeholders(len(args)) + "); END;"

	rows, err := DB.QueryContext(ctx, call, args...)
	elapsed := time.Since(start).Milliseconds()

	metrics.DBSPDuration.WithLabelValues(procName).Observe(float64(elapsed))
	metrics.DBSPTotal.WithLabelValues(procName).Inc()

	if err != nil {
		metrics.DBSPErrors.WithLabelValues(procName, err.Error()).Inc()
		slog.Error("[DBExecQuery]", "Критическая ошибка выполнения", procName, "parameters", placeholders(len(args)), "err", err)
	}

	return rows, err
}

func DBExecNamed(ctx context.Context, query string, procName string, args ...any) error {
	start := time.Now()

	_, err := DB.ExecContext(ctx, query, args...)
	elapsed := time.Since(start).Milliseconds()

	metrics.DBSPDuration.WithLabelValues(procName).Observe(float64(elapsed))
	metrics.DBSPTotal.WithLabelValues(procName).Inc()

	if err != nil {
		metrics.DBSPErrors.WithLabelValues(procName, err.Error()).Inc()
		slog.Error("[DBExecNamed]", "Критическая ошибка выполнения", query, "parameters", placeholders(len(args)), "err", err)
	}

	return err
}

func DBSelectOne(ctx context.Context, name string, dest any, query string, args ...any) error {
	start := time.Now()

	err := DB.QueryRowContext(ctx, query, args...).Scan(dest)
	elapsed := time.Since(start).Milliseconds()

	metrics.DBSelectDuration.WithLabelValues(name).Observe(float64(elapsed))
	metrics.DBSelectTotal.WithLabelValues(name).Inc()

	if err != nil {
		metrics.DBSelectErrors.WithLabelValues(name, err.Error()).Inc()
		slog.Error("[DBSelectOne]", "Критическая ошибка выполнения", name, "query", query, "err", err)
	}

	return err
}

func DBSelectMany[T any](ctx context.Context, name string, dest *[]T, query string, args ...any) error {
	start := time.Now()

	err := DB.SelectContext(ctx, dest, query, args...)
	elapsed := time.Since(start).Milliseconds()

	metrics.DBSelectDuration.WithLabelValues(name).Observe(float64(elapsed))

	metrics.DBSelectTotal.WithLabelValues(name).Inc()

	if err != nil {
		metrics.DBSelectErrors.WithLabelValues(name, err.Error()).Inc()
		slog.Error("[DBSelectMany]", "Критическая ошибка выполнения", name, "query", query, "err", err)
	}

	return err
}
