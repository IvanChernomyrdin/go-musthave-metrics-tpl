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
	service "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
)

func main() {
	//загрузили тут flag и env
	cfg := config.Load()

	db.Init(cfg.DatabaseDSN)

	repo := memory.New()
	svc := service.NewMetricsService(repo)

	//если есть загружаем метрики
	if cfg.Restore {
		if err := svc.LoadFromFile(cfg.FileStoragePath); err != nil {
			log.Printf("Ошибка загрузки метрик: %v", err)
		}
	}
	h := httpserver.NewHandler(svc)
	r := httpserver.NewRouter(h)

	var ticker *time.Ticker

	if cfg.StoreInterval > 0 {
		// Периодическое сохранение
		DurationStoreInterval := time.Duration(cfg.StoreInterval) * time.Second      //привели к времени
		ticker = svc.StartPeriodicSaving(cfg.FileStoragePath, DurationStoreInterval) //выставили тайминги на периоды сохран
		log.Printf("Периодическое сохранение каждые %d секунд", cfg.StoreInterval)
	} else {
		// Синхронное сохранение
		r = svc.SaveOnUpdateMiddleware(cfg.FileStoragePath)(r)
		log.Println("Синхронное сохранение включено")
	}

	server := &http.Server{
		Addr:    cfg.Address,
		Handler: r,
	}

	// реализация graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Сервер запущен на %s", cfg.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка сервера: %v", err)
		}
	}()

	<-quit
	log.Println("Завершение работы сервера...")

	if ticker != nil {
		ticker.Stop()
	}

	log.Println("Сохранение метрик...")
	if err := svc.SaveToFile(cfg.FileStoragePath); err != nil {
		log.Printf("Ошибка сохранения при завершении: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Принудительное завершение: %v", err)
	}

	log.Println("Сервер остановлен")
}
