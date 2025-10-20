package memory

import (
	"sync"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
)

type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

type Storage interface {
	UpsertGauge(name string, v float64) error
	UpsertCounter(name string, d int64) error
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAll() (map[string]float64, map[string]int64)
	UpdateMetricsBatch(metrics []model.Metrics) error
	Close() error
}

func New() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}
func (m *MemStorage) UpsertGauge(id string, value float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.gauges[id] = value
	return nil
}

func (m *MemStorage) UpsertCounter(id string, delta int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[id] += delta
	return nil
}

func (m *MemStorage) GetGauge(name string) (float64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.gauges[name]
	return v, ok
}

func (m *MemStorage) GetCounter(name string) (int64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.counters[name]
	return v, ok
}

func (m *MemStorage) GetAll() (map[string]float64, map[string]int64) {
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
func (m *MemStorage) UpdateMetricsBatch(metrics []model.Metrics) error {
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
