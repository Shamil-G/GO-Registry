//go:build linux || darwin

// logging/logger.go

package logging

import (
	"log/slog"
	"os"

	"github.com/natefinch/lumberjack"
)

// Глобальный атомарный уровень логирования
var LogVar = &slog.LevelVar{}

func Init(IsProduction bool) {
	// Включаем поддержку ANSI-цветов для Windows
	// Создаём каталог logs, если его нет
	os.MkdirAll("logs", 0755)

	// По умолчанию ставим INFO, пока не прочитали конфиг
	LogVar.Set(slog.LevelInfo)

	// Основной лог с ротацией
	mainLog := &lumberjack.Logger{
		Filename:   "logs/registry.log",
		MaxSize:    2, // MB
		MaxBackups: 3,
		MaxAge:     3, // days
		Compress:   true,
	}

	// Лог ошибок с ротацией
	errorLog := &lumberjack.Logger{
		Filename:   "logs/error.log",
		MaxSize:    2,
		MaxBackups: 3,
		MaxAge:     3,
		Compress:   true,
	}

	// Цветной вывод в консоль
	console := NewColorHandler(&slog.HandlerOptions{
		Level: LogVar,
	})

	// JSON‑лог в файл (всё)
	jsonFile := slog.NewJSONHandler(mainLog, &slog.HandlerOptions{
		Level: LogVar,
	})

	// JSON‑лог ошибок (только ERROR и выше)
	jsonError := NewLevelFilter(
		slog.LevelError,
		slog.NewJSONHandler(errorLog, nil),
	)

	handler := NewMultiHandler(console, jsonFile, jsonError)
	slog.SetDefault(slog.New(handler))

}

// Функция для обновления уровня после загрузки .env
func SetLevel(IsProduction bool) {
	var level slog.Level
	if IsProduction {
		level = ParseLogLevel(os.Getenv("PROD_LOG_LEVEL"))
	} else {
		level = ParseLogLevel(os.Getenv("DEV_LOG_LEVEL"))
	}
	LogVar.Set(level)
	slog.Info("Уровень логирования успешно обновлен", "level", level.String())
}
