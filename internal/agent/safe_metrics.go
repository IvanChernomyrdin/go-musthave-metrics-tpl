// Package agent
package agent

import (
	"sync"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
)

// generate:reset
type SafeMetrics struct {
	mu      sync.RWMutex
	metrics []model.Metrics
}

func NewSafeMetrics() *SafeMetrics {
	return &SafeMetrics{
		metrics: make([]model.Metrics, 0),
	}
}

func (sm *SafeMetrics) SetMetrics(metrics []model.Metrics) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.metrics = metrics
}

func (sm *SafeMetrics) GetAndClear() []model.Metrics {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	metrics := sm.metrics
	sm.metrics = make([]model.Metrics, 0)
	return metrics
}

func (sm *SafeMetrics) Append(metrics []model.Metrics) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.metrics = append(sm.metrics, metrics...)
}

func (sm *SafeMetrics) Len() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.metrics)
}
