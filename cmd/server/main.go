package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/config"
	db "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/config/db"
	httpserver "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/handler"
	memory "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/repository/memory"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/repository/postgres"
	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/runtime"
	service "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
)

func main() {
	cfg := config.Load()

	castomLogger := logger.NewHTTPLogger().Logger.Sugar()

	var repo memory.Storage
	var usePostgreSQL bool

	// Пытаемся использовать PostgreSQL если указан DSN
	if cfg.DatabaseDSN != "" {
		if err := db.Init(cfg.DatabaseDSN); err != nil {
			castomLogger.Infof("PostgreSQL недоступна: %v", err)
			repo = memory.New()
			usePostgreSQL = false
		} else {
			repo = postgres.New()
			usePostgreSQL = true
			castomLogger.Info("Используется PostgreSQL хранилище")
		}
	} else {
		repo = memory.New()
		usePostgreSQL = false
		castomLogger.Info("Используется memory хранилище")
	}
	defer func() {
		if err := repo.Close(); err != nil {
			castomLogger.Infof("Ошибка при закрытии хранилища: %v", err)
		}
	}()

	svc := service.NewMetricsService(repo)

	// Загрузка из файла только если НЕ используется PostgreSQL
	if !usePostgreSQL && cfg.Restore && cfg.FileStoragePath != "" {
		castomLogger.Infof("Загрузка метрик из файла: %s", cfg.FileStoragePath)
		if err := svc.LoadFromFile(cfg.FileStoragePath); err != nil {
			castomLogger.Infof("Ошибка загрузки метрик: %v", err)
		}
	}

	h := httpserver.NewHandler(svc)
	r := httpserver.NewRouter(h, cfg.HashKey)

	var ticker *time.Ticker

	// Настройка сохранения в файл только если НЕ используется PostgreSQL
	if !usePostgreSQL && cfg.FileStoragePath != "" {
		if cfg.StoreInterval > 0 {
			DurationStoreInterval := time.Duration(cfg.StoreInterval) * time.Second
			ticker = svc.StartPeriodicSaving(cfg.FileStoragePath, DurationStoreInterval)
			castomLogger.Infof("Периодическое сохранение каждые %d секунд", cfg.StoreInterval)
		} else {
			r = svc.SaveOnUpdateMiddleware(cfg.FileStoragePath)(r)
			castomLogger.Info("Синхронное сохранение включено")
		}
	}

	server := &http.Server{
		Addr:    cfg.Address,
		Handler: r,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Сервер запущен на %s", cfg.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			castomLogger.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	<-quit
	castomLogger.Info("Завершение работы сервера...")

	if ticker != nil {
		ticker.Stop()
	}

	// Сохранение в файл только если НЕ используется PostgreSQL
	if !usePostgreSQL && cfg.FileStoragePath != "" {
		castomLogger.Info("Сохранение метрик...")
		if err := svc.SaveToFile(cfg.FileStoragePath); err != nil {
			castomLogger.Infof("Ошибка сохранения при завершении: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		castomLogger.Fatalf("Принудительное завершение: %v", err)
	}

	castomLogger.Info("Сервер остановлен")
}
