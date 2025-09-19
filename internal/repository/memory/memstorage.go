package memory

import "sync"

type MemStorage struct {
	mu       sync.RWMutex
	gauges   map[string]float64
	counters map[string]int64
}

type Storage interface {
	UpsertGauge(name string, v float64)
	UpsertCounter(name string, d int64)
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAll() (map[string]float64, map[string]int64)
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
