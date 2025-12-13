package main

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	config "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/config"
	db "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/config/db"
	httpserver "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/handler"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/middleware"
	memory "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/repository/memory"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/repository/postgres"
	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/runtime"
	service "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
)

func main() {
	cfg := config.Load()

	customLogger := logger.NewHTTPLogger().Logger.Sugar()

	go func() {
		http.ListenAndServe("localhost:6061", nil)
	}()

	var repo memory.Storage
	var usePostgreSQL bool

	// Пытаемся использовать PostgreSQL если указан DSN
	if cfg.DatabaseDSN != "" {
		if err := db.Init(cfg.DatabaseDSN); err != nil {
			customLogger.Infof("PostgreSQL недоступна: %v", err)
			repo = memory.New()
			usePostgreSQL = false
		} else {
			repo = postgres.New()
			usePostgreSQL = true
			customLogger.Info("Используется PostgreSQL хранилище")
		}
	} else {
		repo = memory.New()
		usePostgreSQL = false
		customLogger.Info("Используется memory хранилище")
	}
	defer func() {
		if err := repo.Close(); err != nil {
			customLogger.Infof("Ошибка при закрытии хранилища: %v", err)
		}
	}()

	svc := service.NewMetricsService(repo)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Загрузка из файла только если НЕ используется PostgreSQL
	if !usePostgreSQL && cfg.Restore && cfg.FileStoragePath != "" {
		customLogger.Infof("Загрузка метрик из файла: %s", cfg.FileStoragePath)
		if err := svc.LoadFromFile(ctx, cfg.FileStoragePath); err != nil {
			customLogger.Infof("Ошибка загрузки метрик: %v", err)
		}
	}

	h := httpserver.NewHandler(svc)
	var auditReceivers []middleware.AuditReceiver
	if cfg.AuditFile != "" {
		auditReceivers = append(auditReceivers, &middleware.FileAuditReceiver{FilePath: cfg.AuditFile})
	}
	if cfg.AuditURL != "" {
		auditReceivers = append(auditReceivers, &middleware.URLAuditReceiver{URL: cfg.AuditURL})
	}
	r := httpserver.NewRouter(h, cfg.HashKey, auditReceivers)

	var ticker *time.Ticker

	// Настройка сохранения в файл только если НЕ используется PostgreSQL
	if !usePostgreSQL && cfg.FileStoragePath != "" {
		if cfg.StoreInterval > 0 {
			DurationStoreInterval := time.Duration(cfg.StoreInterval) * time.Second
			ticker = svc.StartPeriodicSaving(ctx, cfg.FileStoragePath, DurationStoreInterval)
			customLogger.Infof("Периодическое сохранение каждые %d секунд", cfg.StoreInterval)
		} else {
			r = svc.SaveOnUpdateMiddleware(cfg.FileStoragePath)(r)
			customLogger.Info("Синхронное сохранение включено")
		}
	}

	server := &http.Server{
		Addr:         cfg.Address,
		Handler:      r,
		ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.IdleTimeout) * time.Second,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Сервер запущен на %s", cfg.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			customLogger.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	<-quit
	customLogger.Info("Завершение работы сервера...")

	if ticker != nil {
		ticker.Stop()
	}

	// Сохранение в файл только если НЕ используется PostgreSQL
	if !usePostgreSQL && cfg.FileStoragePath != "" {
		customLogger.Info("Сохранение метрик...")
		if err := svc.SaveToFile(ctx, cfg.FileStoragePath); err != nil {
			customLogger.Infof("Ошибка сохранения при завершении: %v", err)
		}
	}

	if err := server.Shutdown(ctx); err != nil {
		customLogger.Fatalf("Принудительное завершение: %v", err)
	}

	customLogger.Info("Сервер остановлен")
}
