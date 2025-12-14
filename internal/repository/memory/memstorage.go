// Package memory предожает реализацию хранилища метрик в оперативной памяти.
// хранилище потокобезопасно и предназначено для использования в текущей версии сервиса для тестов т.к. при перезагрузке или падении сервиса данные потеряются.
package memory

import (
	"context"
	"sync"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
)

// реализует хранилище метрик в оперативной памяти.
// использует мапы для хранения чтобы значения были уникальными.
// добавлены мьютексы для потокобезопасности.
type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

// определяет интерфейс хранилища метрик.
type Storage interface {
	// создает или обновляет метрику типа gauge.
	UpsertGauge(ctx context.Context, name string, v float64) error
	// создает или обновляет метрику типа counter.
	UpsertCounter(ctx context.Context, name string, d int64) error
	// возвращает значение метрики типа gauge по её имени.
	GetGauge(ctx context.Context, name string) (float64, bool)
	// возвращает значение метрики типа counter по её имени.
	GetCounter(ctx context.Context, name string) (int64, bool)
	// возвращает весь список метрик gauge и counter
	GetAll(ctx context.Context) (map[string]float64, map[string]int64)
	// обновляет несколько метрик за одну операцию.
	UpdateMetricsBatch(ctx context.Context, metrics []model.Metrics) error
	// освобождает ресурсы хранилища.
	Close() error
}

func New() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (m *MemStorage) UpsertGauge(ctx context.Context, id string, value float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[id] = value
	return nil
}

func (m *MemStorage) UpsertCounter(ctx context.Context, id string, delta int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[id] += delta
	return nil
}

func (m *MemStorage) GetGauge(ctx context.Context, name string) (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.gauges[name]
	return v, ok
}

func (m *MemStorage) GetCounter(ctx context.Context, name string) (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.counters[name]
	return v, ok
}

func (m *MemStorage) GetAll(ctx context.Context) (map[string]float64, map[string]int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	gs := make(map[string]float64, len(m.gauges))
	for key, val := range m.gauges {
		gs[key] = val
	}

	cs := make(map[string]int64, len(m.counters))
	for key, val := range m.counters {
		cs[key] = val
	}

	return gs, cs
}
func (m *MemStorage) UpdateMetricsBatch(ctx context.Context, metrics []model.Metrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, metric := range metrics {
		switch metric.MType {
		case model.Gauge:
			if metric.Value != nil {
				m.gauges[metric.ID] = *metric.Value
			}
		case model.Counter:
			if metric.Delta != nil {
				m.counters[metric.ID] += *metric.Delta
			}
		}
	}
	return nil
}
func (m *MemStorage) Close() error {
	return nil
}
