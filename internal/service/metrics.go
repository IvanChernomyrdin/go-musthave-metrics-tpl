package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

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

func (s *MetricsService) GetValue(mtype, name string) (string, bool, bool) {
	switch mtype {
	case Gauge:
		if val, ok := s.repo.GetGauge(name); ok {
			out := strconv.FormatFloat(val, 'f', 3, 64)
			out = strings.TrimRight(out, "0")
			out = strings.TrimRight(out, ".")
			return out, true, true
		}
		return "", false, true
	case Counter:
		if val, ok := s.repo.GetCounter(name); ok {
			return strconv.FormatInt(val, 10), true, true
		}
		return "", false, true
	default:
		return "", false, false
	}
}

func (s *MetricsService) AllText() map[string]string {
	gs, cs := s.repo.GetAll()
	out := make(map[string]string, len(gs)+len(cs))

	for key, val := range gs {
		out[fmt.Sprintf("%s.%s", Gauge, key)] = strconv.FormatFloat(val, 'f', -1, 64)
	}
	for key, val := range cs {
		out[fmt.Sprintf("%s.%s", Counter, key)] = fmt.Sprintf("%d", val)
	}

	return out
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (s *MetricsService) SaveToFile(filename string) error {
	gauges, counters := s.repo.GetAll()

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
	tmpFilename := filename + ".tmp"
	file, err := os.OpenFile(tmpFilename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(metrics); err != nil {
		file.Close()
		os.Remove(tmpFilename)
		return fmt.Errorf("failed to encode metrics: %w", err)
	}
	if err := file.Close(); err != nil {
		os.Remove(tmpFilename)                                 // ДОБАВЛЕНО: удаляем временный файл при ошибке закрытия
		return fmt.Errorf("failed to close tmp file: %w", err) // ИСПРАВЛЕНО: опечатка "fule" -> "file"
	}
	if err := os.Rename(tmpFilename, filename); err != nil {
		os.Remove(tmpFilename)
		return fmt.Errorf("failed to rename tmp file: %w", err)
	}
	return nil
}

func (s *MetricsService) LoadFromFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		// если файла нет то ничего не возвращаем
		if os.IsNotExist(err) {
			return nil
		}
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
				if err := s.repo.UpsertGauge(metric.ID, *metric.Value); err != nil {
					return fmt.Errorf("failed to restore gauge %s: %w", metric.ID, err)
				}
			}
		case Counter:
			if metric.Delta != nil {
				if err := s.repo.UpsertCounter(metric.ID, *metric.Delta); err != nil {
					return fmt.Errorf("failed to restore counter %s: %w", metric.ID, err) // ИСПРАВЛЕНО: "gauge" -> "counter"
				}
			}
		}
	}
	log.Printf("Loaded metrics from %s", filename)
	return nil
}

func (s *MetricsService) SaveOnUpdateMiddleware(filename string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { // ИСПРАВЛЕНО: next вместо h для согласованности
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			next.ServeHTTP(rw, r) // ИСПРАВЛЕНО: next вместо h

			if r.Method == http.MethodPost &&
				(strings.HasPrefix(r.URL.Path, "/update/") || r.URL.Path == "/update") &&
				rw.statusCode == http.StatusOK {
				if err := s.SaveToFile(filename); err != nil {
					log.Printf("Error saving metrics synchronously: %v", err)
				}
			}
		})
	}
}
