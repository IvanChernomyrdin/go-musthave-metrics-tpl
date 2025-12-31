package main

import (
	"context"
	"fmt"
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
	service "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/pgk/logger"
)

var (
	buildVersion string
	buildDate    string
	buildCommit  string
)

func defaultIfEmpty(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func main() {
	fmt.Printf("Build version: %s\n", defaultIfEmpty(buildVersion))
	fmt.Printf("Build date: %s\n", defaultIfEmpty(buildDate))
	fmt.Printf("Build commit: %s\n", defaultIfEmpty(buildCommit))
	cfg := config.Load()
	customLogger := logger.NewHTTPLogger().Logger.Sugar()
	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	go func() {
		http.ListenAndServe("localhost:6061", nil)
	}()

	var repo memory.Storage
	var usePostgreSQL bool

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

	if !usePostgreSQL && cfg.Restore && cfg.FileStoragePath != "" {
		loadCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		customLogger.Infof("Загрузка метрик из файла: %s", cfg.FileStoragePath)
		if err := svc.LoadFromFile(loadCtx, cfg.FileStoragePath); err != nil {
			customLogger.Infof("Ошибка загрузки метрик: %v", err)
		}
		cancel()
	}

	h := httpserver.NewHandler(svc)
	var auditReceivers []middleware.AuditReceiver
	if cfg.AuditFile != "" {
		auditReceivers = append(auditReceivers, &middleware.FileAuditReceiver{FilePath: cfg.AuditFile})
	}
	if cfg.AuditURL != "" {
		auditReceivers = append(auditReceivers, &middleware.URLAuditReceiver{URL: cfg.AuditURL})
	}
	r := httpserver.NewRouter(h, cfg.HashKey, auditReceivers, cfg.CryptoKey)

	var ticker *time.Ticker
	if !usePostgreSQL && cfg.FileStoragePath != "" {
		if cfg.StoreInterval > 0 {
			d := time.Duration(cfg.StoreInterval) * time.Second
			ticker = svc.StartPeriodicSaving(appCtx, cfg.FileStoragePath, d)
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

	errCh := make(chan error, 1)
	go func() {
		customLogger.Infof("Сервер запущен на %s", cfg.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-appCtx.Done():
		customLogger.Info("Получен сигнал завершения, останавливаем сервер...")
	case err := <-errCh:
		if err != nil {
			customLogger.Fatalf("Ошибка сервера: %v", err)
		}
		customLogger.Info("Сервер завершился")
		return
	}

	if ticker != nil {
		ticker.Stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		customLogger.Fatalf("Принудительное завершение: %v", err)
	}

	if !usePostgreSQL && cfg.FileStoragePath != "" {
		customLogger.Info("Сохранение метрик...")
		if err := svc.SaveToFile(shutdownCtx, cfg.FileStoragePath); err != nil {
			customLogger.Errorf("Ошибка сохранения при завершении: %v", err)
		}
	}

	customLogger.Info("Сервер остановлен")
}
