package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	httpserver "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/handler"
	memory "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/repository/memory"
	service "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
)

var (
	addrServer      string
	storeInterval   int
	fileStoragePath string
	restore         bool
)

func init() {
	flag.StringVar(&addrServer, "a", "localhost:8080", "http-server address")
	flag.IntVar(&storeInterval, "i", 10, "interval in seconds to save metrics to disk (0 for synchronous write)")
	flag.StringVar(&fileStoragePath, "f", "/tmp/metrics.json", "path to file for storing metrics")
	flag.BoolVar(&restore, "r", true, "load saved metrics on startup")
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntWithDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBoolWithDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func main() {
	flag.Parse()

	addr := getEnvWithDefault("ADDRESS", addrServer)
	storeInterval = getEnvIntWithDefault("STORE_INTERVAL", storeInterval)
	fileStoragePath = getEnvWithDefault("FILE_STORAGE_PATH", fileStoragePath)
	restore = getEnvBoolWithDefault("RESTORE", restore)

	repo := memory.New()
	svc := service.NewMetricsService(repo)

	// загружаем метрики из файла
	if restore {
		if err := svc.LoadFromFile(fileStoragePath); err != nil {
			log.Printf("Error loading metrics data: %v", err)
		} else {
			log.Printf("Successfully loaded metrics from %s", fileStoragePath)
		}
	}

	// Создаем handler и router
	handler := httpserver.NewHandler(svc)
	router := httpserver.NewRouter(handler)

	// Для синхронного сохранения добавляем middleware
	if storeInterval == 0 {
		// Обертываем router в middleware
		routerWithMiddleware := svc.SaveOnUpdateMiddleware(fileStoragePath)(router)
		fmt.Printf("Сервер запущен на %s\n", addr)
		log.Fatal(http.ListenAndServe(addr, routerWithMiddleware))
	} else {
		// сохранение в файл по таймеру
		go func() {
			ticker := time.NewTicker(time.Duration(storeInterval) * time.Second)
			defer ticker.Stop()

			for range ticker.C {
				if err := svc.SaveToFile(fileStoragePath); err != nil {
					log.Printf("Error saving metrics: %v", err)
				} else {
					log.Printf("Metrics saved to %s", fileStoragePath)
				}
			}
		}()
		log.Printf("Periodic saving enabled every %d seconds", storeInterval)

		fmt.Printf("Сервер запущен на %s\n", addr)
		log.Fatal(http.ListenAndServe(addr, router))
	}
}
