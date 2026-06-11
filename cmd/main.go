// main.go

package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"gusseynov/GO-Registry/config"
	"gusseynov/GO-Registry/logging"
	"gusseynov/GO-Registry/metrics"
	"gusseynov/GO-Registry/sso"
	"gusseynov/GO-Registry/storage"
)

var IsProduction bool

func initLogger() {
	f, err := os.OpenFile("logs/registry.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelInfo, // минимальный уровень логирования
	})

	slog.SetDefault(slog.New(handler))
}

func main() {
	IsProduction = true
	runMode := "Production"
	if runtime.GOOS == "windows" {
		IsProduction = false
		runMode = "Development"
	}
	logging.Init(IsProduction) // ← ВАЖНО: включаем slog + цвета + ротацию + error.log
	// initLogger()
	logging.Start("Registry started", "Mode", runMode)

	// 1. Инициализируем твой глобальный конфиг из .env
	if err := config.LoadConfig(IsProduction); err != nil {
		slog.Error("Критическая ошибка загрузки конфигурации", "Error", err)
		os.Exit(1)
	}
	logging.Start("REGISTRY Config ./env loaded ...")
	// 3. Динамически обновляем уровень логирования на тот, что пришел из ENV
	logging.SetLevel(IsProduction)

	startMetrics := time.Now()
	metrics.Init()
	logging.Start("REGISTRY Metrics started ...", "duration", time.Since(startMetrics))

	startDB := time.Now()
	if err := storage.Init(); err != nil {
		slog.Warn("Предупреждение: База Oracle недоступна, работаем автономно", "error", err)
	}
	logging.Start("DB Client started ...", "duration", time.Since(startDB))

	startSSO := time.Now()
	sso.Init()
	logging.Start("SSO Client started ...", "duration", time.Since(startSSO))

	chi := Router()

	srv := &http.Server{
		Addr:    config.Cfg.ListenAddr,
		Handler: chi,
	}

	go func() {
		logging.Start("Start listening", "addr", config.Cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Start("HTTP server error", "Error", err)
		}
	}()
	logging.Start("REGISTRY HTTP started ...", "ADDR", config.Cfg.ListenAddr)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	<-quit
	logging.Start("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Warn("Server forced to shutdown", "Warning", err)
	}

	logging.Start("Server REGISTRY exited cleanly")
}
