//go:build windows

// logging/logger.go

package logging

import (
	"log/slog"
	"os"
	"syscall"
	"unsafe"

	"github.com/natefinch/lumberjack"
)

// Глобальный атомарный уровень логирования
var LogVar = &slog.LevelVar{}

func enableWindowsStdoutColor() {
	// STD_OUTPUT_HANDLE для Windows равен -11 (или 4294967285 в uint32)
	const stdOutputHandle = uint32(0xFFFFFFF5)
	const enableVirtualTerminalProcessing = 0x0004

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	procGetStdHandle := kernel32.NewProc("GetStdHandle")
	procGetConsoleMode := kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode := kernel32.NewProc("SetConsoleMode")

	handle, _, _ := procGetStdHandle.Call(uintptr(stdOutputHandle))
	if handle == uintptr(syscall.InvalidHandle) {
		return
	}

	var mode uint32
	// Передаем uintptr от указателя на mode
	_, _, _ = procGetConsoleMode.Call(handle, uintptr(unsafe.Pointer(&mode)))

	mode |= enableVirtualTerminalProcessing
	_, _, _ = procSetConsoleMode.Call(handle, uintptr(mode))
}

func Init(IsProduction bool) {
	// Включаем поддержку ANSI-цветов для Windows
	if !IsProduction {
		enableWindowsStdoutColor()
	}
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

	// JSON‑лог в файл
	jsonFile := slog.NewJSONHandler(mainLog, &slog.HandlerOptions{
		Level: LogVar,
	})

	// JSON‑лог ошибок
	jsonError := NewLevelFilter(
		slog.LevelError,
		slog.NewJSONHandler(errorLog, nil),
	)

	// MultiHandler (консоль + файл + error.log)
	handler := NewMultiHandler(console, jsonFile, jsonError)

	// Устанавливаем глобальный логгер
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
