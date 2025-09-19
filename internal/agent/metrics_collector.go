package agent

import (
	"math/rand"
	"runtime"
	"sync"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
)

type RuntimeMetricsCollector struct {
	pollCount int64
	mu        sync.Mutex
}

// NewRuntimeMetricsCollector создает сборщик метрик
func NewRuntimeMetricsCollector() *RuntimeMetricsCollector {
	return &RuntimeMetricsCollector{
		pollCount: 0,
	}
}

func (rmc *RuntimeMetricsCollector) Collect() []model.Metrics {
	rmc.mu.Lock()
	defer rmc.mu.Unlock()

	rmc.pollCount++

	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	// выделяем память для всех метрик - 29 шт
	metrics := make([]model.Metrics, 0, 29)

	addGauge := func(id string, value float64) {
		val := value
		metrics = append(metrics, model.Metrics{
			ID:    id,
			MType: model.Gauge,
			Value: &val,
			Hash:  "",
		})
	}
	addCounter := func(id string, value int64) {
		val := value
		metrics = append(metrics, model.Metrics{
			ID:    id,
			MType: model.Counter,
			Delta: &val,
			Hash:  "",
		})
	}

	// Метрики из runtime (gauge)
	addGauge("Alloc", float64(stats.Alloc))
	addGauge("BuckHashSys", float64(stats.BuckHashSys))
	addGauge("Frees", float64(stats.Frees))
	addGauge("GCCPUFraction", float64(stats.GCCPUFraction))
	addGauge("GCSys", float64(stats.GCSys))
	addGauge("HeapAlloc", float64(stats.HeapAlloc))
	addGauge("HeapIdle", float64(stats.HeapIdle))
	addGauge("HeapInuse", float64(stats.HeapInuse))
	addGauge("HeapObjects", float64(stats.HeapObjects))
	addGauge("HeapReleased", float64(stats.HeapReleased))
	addGauge("HeapSys", float64(stats.HeapSys))
	addGauge("LastGC", float64(stats.LastGC))
	addGauge("Lookups", float64(stats.Lookups))
	addGauge("MCacheInuse", float64(stats.MCacheInuse))
	addGauge("MCacheSys", float64(stats.MCacheSys))
	addGauge("MSpanInuse", float64(stats.MSpanInuse))
	addGauge("MSpanSys", float64(stats.MSpanSys))
	addGauge("Mallocs", float64(stats.Mallocs))
	addGauge("NextGC", float64(stats.NextGC))
	addGauge("NumForcedGC", float64(stats.NumForcedGC))
	addGauge("NumGC", float64(stats.NumGC))
	addGauge("OtherSys", float64(stats.OtherSys))
	addGauge("PauseTotalNs", float64(stats.PauseTotalNs))
	addGauge("StackInuse", float64(stats.StackInuse))
	addGauge("StackSys", float64(stats.StackSys))
	addGauge("Sys", float64(stats.Sys))
	addGauge("TotalAlloc", float64(stats.TotalAlloc))

	// Дополнительные метрики
	addCounter("PollCount", rmc.pollCount)
	addGauge("RandomValue", rand.Float64())

	return metrics
}
