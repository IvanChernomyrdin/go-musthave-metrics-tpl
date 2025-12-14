// Package service содержит бизнес-логику приложения для работы с метриками.
// Он служит промежуточным слоем между http-обработчиками и хранилищем данных(memory, database, json file).
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/runtime"
)

var customLogger = logger.NewHTTPLogger().Logger.Sugar()

// константы определяющие типы метрик.
const (
	Counter string = "counter" // тип-метрики счётчика (целочисленное).
	Gauge   string = "gauge"   // тип-метрики измерителя (число с плавающей точкой).
)

// интерфейс для работы с хранилищем метрик.
type MetricsRepo interface {
	// создаёт или обновляет метрику типа gauge.
	UpsertGauge(ctx context.Context, id string, value float64) error
	// создаёт или обновляет метрику типа counter.
	UpsertCounter(ctx context.Context, id string, delta int64) error
	// получает значение метрики типа gauge.
	GetGauge(ctx context.Context, id string) (float64, bool)
	// получает значение метрики типа counter.
	GetCounter(ctx context.Context, id string) (int64, bool)
	// получает список метрик gauge и counter.
	GetAll(ctx context.Context) (map[string]float64, map[string]int64)
	// обновляет или добавляет несколько метрик за одну операцию.
	UpdateMetricsBatch(ctx context.Context, metrics []model.Metrics) error
}

// предостовляет бизнес-логику для работы с метриками.
// прослойка между http-обработчиками и бд.
type MetricsService struct {
	repo MetricsRepo
}

// создаёт новый экземпляр MetricsService.
func NewMetricsService(repo MetricsRepo) *MetricsService {
	return &MetricsService{repo: repo}
}

// обновляет метрику типа gauge.
func (ms *MetricsService) UpdateGauge(ctx context.Context, id string, value float64) error {
	return ms.repo.UpsertGauge(ctx, id, value)
}

// обновляет метрику типа counter
func (ms *MetricsService) UpdateCounter(ctx context.Context, id string, delta int64) error {
	return ms.repo.UpsertCounter(ctx, id, delta)
}

// получение значения метрики типа gauge
func (ms *MetricsService) GetGauge(ctx context.Context, id string) (float64, bool) {
	return ms.repo.GetGauge(ctx, id)
}

// получение значения метрики типа counter
func (ms *MetricsService) GetCounter(ctx context.Context, id string) (int64, bool) {
	return ms.repo.GetCounter(ctx, id)
}

// получение значения метрики
// в ответе возвращает: три значения.
// первое значение: строковое представление знаячения.
// второе значение: булево значение найдена ли метрика.
// третье значение: булево значение корректен ли тип метрики.
func (ms *MetricsService) GetValue(ctx context.Context, mtype, name string) (string, bool, bool) {
	switch mtype {
	case Gauge:
		if val, ok := ms.repo.GetGauge(ctx, name); ok {
			out := strconv.FormatFloat(val, 'f', 3, 64)
			out = strings.TrimRight(out, "0")
			out = strings.TrimRight(out, ".")
			return out, true, true
		}
		return "", false, true
	case Counter:
		if val, ok := ms.repo.GetCounter(ctx, name); ok {
			return strconv.FormatInt(val, 10), true, true
		}
		return "", false, true
	default:
		return "", false, false
	}
}

// возвращает все метрики в виде карты "тип": "значение".
func (ms *MetricsService) AllText(ctx context.Context) map[string]string {
	gs, cs := ms.repo.GetAll(ctx)
	out := make(map[string]string, len(gs)+len(cs))

	for key, val := range gs {
		out[fmt.Sprintf("%s.%s", Gauge, key)] = strconv.FormatFloat(val, 'f', -1, 64)
	}
	for key, val := range cs {
		out[fmt.Sprintf("%s.%s", Counter, key)] = fmt.Sprintf("%d", val)
	}

	return out
}

// обновляет несколько метрик за одну операцию.
func (ms *MetricsService) UpdateMetricsBatch(ctx context.Context, metrics []model.Metrics) error {
	return ms.repo.UpdateMetricsBatch(ctx, metrics)
}

// сохраняет все метрики в JSON файл.
// сохранение происходит атомарно через временный файл.
// если filename пустой, функция ничего не делает.
func (ms *MetricsService) SaveToFile(ctx context.Context, filename string) error {
	if filename == "" {
		return nil
	}

	// СОЗДАЕМ ДИРЕКТОРИЮ ЕСЛИ НЕ СУЩЕСТВУЕТ
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	gauges, counters := ms.repo.GetAll(ctx)
	var metrics []model.Metrics

	for id, value := range gauges {
		v := value
		metrics = append(metrics, model.Metrics{
			ID:    id,
			MType: Gauge,
			Value: &v,
		})
	}

	for id, delta := range counters {
		d := delta
		metrics = append(metrics, model.Metrics{
			ID:    id,
			MType: Counter,
			Delta: &d,
		})
	}

	// Атомарное сохранение через временный файл
	tmpFilename := filename + ".tmp"
	file, err := os.Create(tmpFilename)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(metrics); err != nil {
		os.Remove(tmpFilename)
		return fmt.Errorf("failed to encode metrics: %w", err)
	}

	// Закрываем файл перед переименованием
	file.Close()

	if err := os.Rename(tmpFilename, filename); err != nil {
		os.Remove(tmpFilename)
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// загружает метрики из JSON файла.
func (ms *MetricsService) LoadFromFile(ctx context.Context, filename string) error {
	if filename == "" {
		return nil
	}

	file, err := os.Open(filename)
	//если файла нет выходим
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var metrics []model.Metrics
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&metrics); err != nil {
		return fmt.Errorf("failed to decode metrics: %w", err)
	}

	for _, metric := range metrics {
		switch metric.MType {
		case Gauge:
			if metric.Value != nil {
				if err := ms.repo.UpsertGauge(ctx, metric.ID, *metric.Value); err != nil {
					return fmt.Errorf("failed to restore gauge %s: %w", metric.ID, err)
				}
			}
		case Counter:
			if metric.Delta != nil {
				if err := ms.repo.UpsertCounter(ctx, metric.ID, *metric.Delta); err != nil {
					return fmt.Errorf("failed to restore counter %s: %w", metric.ID, err)
				}
			}
		}
	}

	log.Printf("Successfully loaded metrics from %s", filename)
	return nil
}

// ResponseWriter для отслеживания статуса ответа
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// переопределяет хедер для отслеживания статуса
func (rw *ResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// SaveOnUpdateMiddleware middleware для синхронного сохранения после обновлений
// если успешно то сохраянет метрику в файл
func (ms *MetricsService) SaveOnUpdateMiddleware(filename string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &ResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(rw, r)

			// Сохраняем после успешного POST запроса к /update
			if r.Method == http.MethodPost &&
				(strings.HasPrefix(r.URL.Path, "/update/") || r.URL.Path == "/update") &&
				rw.statusCode == http.StatusOK {
				if err := ms.SaveToFile(r.Context(), filename); err != nil {
					customLogger.Warnf("Error saving metrics synchronously: %v", err)
				}
			}
		})
	}
}

// StartPeriodicSaving запускает периодическое сохранение метрик
func (ms *MetricsService) StartPeriodicSaving(ctx context.Context, filename string, interval time.Duration) *time.Ticker {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := ms.SaveToFile(ctx, filename); err != nil {
					customLogger.Warnf("Error during periodic save: %v", err)
				} else {
					customLogger.Infof("Metrics saved to %s", filename)
				}
			case <-ctx.Done():
				customLogger.Warnf("Periodic saving stopped: %v", ctx.Err())
				return
			}
		}
	}()
	return ticker
}
