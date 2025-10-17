package service

import (
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
)

const (
	Counter = "counter"
	Gauge   = "gauge"
)

type MetricsRepo interface {
	UpsertGauge(id string, value float64) error
	UpsertCounter(id string, delta int64) error
	GetGauge(id string) (float64, bool)
	GetCounter(id string) (int64, bool)
	GetAll() (map[string]float64, map[string]int64)
}

type MetricsService struct {
	repo MetricsRepo
}

func NewMetricsService(repo MetricsRepo) *MetricsService {
	return &MetricsService{repo: repo}
}

func (ms *MetricsService) UpdateGauge(id string, value float64) error {
	return ms.repo.UpsertGauge(id, value)
}

func (ms *MetricsService) UpdateCounter(id string, delta int64) error {
	return ms.repo.UpsertCounter(id, delta)
}

func (ms *MetricsService) GetGauge(id string) (float64, bool) {
	return ms.repo.GetGauge(id)
}

func (ms *MetricsService) GetCounter(id string) (int64, bool) {
	return ms.repo.GetCounter(id)
}

func (ms *MetricsService) GetValue(mtype, name string) (string, bool, bool) {
	switch mtype {
	case Gauge:
		if val, ok := ms.repo.GetGauge(name); ok {
			out := strconv.FormatFloat(val, 'f', 3, 64)
			out = strings.TrimRight(out, "0")
			out = strings.TrimRight(out, ".")
			return out, true, true
		}
		return "", false, true
	case Counter:
		if val, ok := ms.repo.GetCounter(name); ok {
			return strconv.FormatInt(val, 10), true, true
		}
		return "", false, true
	default:
		return "", false, false
	}
}

func (ms *MetricsService) AllText() map[string]string {
	gs, cs := ms.repo.GetAll()
	out := make(map[string]string, len(gs)+len(cs))

	for key, val := range gs {
		out[fmt.Sprintf("%s.%s", Gauge, key)] = strconv.FormatFloat(val, 'f', -1, 64)
	}
	for key, val := range cs {
		out[fmt.Sprintf("%s.%s", Counter, key)] = fmt.Sprintf("%d", val)
	}

	return out
}

func (ms *MetricsService) SaveToFile(filename string) error {
	if filename == "" {
		return nil
	}

	// СОЗДАЕМ ДИРЕКТОРИЮ ЕСЛИ НЕ СУЩЕСТВУЕТ
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	gauges, counters := ms.repo.GetAll()
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

func (ms *MetricsService) LoadFromFile(filename string) error {
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
				if err := ms.repo.UpsertGauge(metric.ID, *metric.Value); err != nil {
					return fmt.Errorf("failed to restore gauge %s: %w", metric.ID, err)
				}
			}
		case Counter:
			if metric.Delta != nil {
				if err := ms.repo.UpsertCounter(metric.ID, *metric.Delta); err != nil {
					return fmt.Errorf("failed to restore counter %s: %w", metric.ID, err)
				}
			}
		}
	}

	log.Printf("Successfully loaded metrics from %s", filename)
	return nil
}

// responseWriter для отслеживания статуса ответа
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// SaveOnUpdateMiddleware middleware для синхронного сохранения после обновлений
func (ms *MetricsService) SaveOnUpdateMiddleware(filename string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(rw, r)

			// Сохраняем после успешного POST запроса к /update
			if r.Method == http.MethodPost &&
				(strings.HasPrefix(r.URL.Path, "/update/") || r.URL.Path == "/update") &&
				rw.statusCode == http.StatusOK {
				if err := ms.SaveToFile(filename); err != nil {
					log.Printf("Error saving metrics synchronously: %v", err)
				}
			}
		})
	}
}

// StartPeriodicSaving запускает периодическое сохранение метрик
func (ms *MetricsService) StartPeriodicSaving(filename string, interval time.Duration) *time.Ticker {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := ms.SaveToFile(filename); err != nil {
				log.Printf("Error during periodic save: %v", err)
			} else {
				log.Printf("Metrics saved to %s", filename)
			}
		}
	}()
	return ticker
}
