package httpserver

import (
	"fmt"
	"testing"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/repository/memory"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
)

func BenchmarkUpdateGauge(b *testing.B) {
	storage := memory.New()
	svc := service.NewMetricsService(storage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.UpdateGauge("test", float64(i))
	}
}

func BenchmarkUpdateCounter(b *testing.B) {
	storage := memory.New()
	svc := service.NewMetricsService(storage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.UpdateCounter("test", int64(i))
	}
}

func BenchmarkUpdateMetricsBatch(b *testing.B) {
	storage := memory.New()
	svc := service.NewMetricsService(storage)

	metrics := []model.Metrics{
		{ID: "test1", MType: model.Gauge, Value: float64Ptr(1.5)},
		{ID: "test2", MType: model.Counter, Delta: int64Ptr(10)},
		{ID: "test3", MType: model.Gauge, Value: float64Ptr(3.14)},
		{ID: "test4", MType: model.Counter, Delta: int64Ptr(5)},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.UpdateMetricsBatch(metrics)
	}
}

func BenchmarkUpdateGaugeDifferentKeys(b *testing.B) {
	storage := memory.New()
	svc := service.NewMetricsService(storage)

	keys := make([]string, 1000)
	for i := range keys {
		keys[i] = fmt.Sprintf("key%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.UpdateGauge(keys[i%1000], float64(i))
	}
}

func BenchmarkUpdateCounterDifferentKeys(b *testing.B) {
	storage := memory.New()
	svc := service.NewMetricsService(storage)

	keys := make([]string, 1000)
	for i := range keys {
		keys[i] = fmt.Sprintf("key%d", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.UpdateCounter(keys[i%1000], int64(i))
	}
}

func float64Ptr(f float64) *float64 { return &f }
func int64Ptr(i int64) *int64       { return &i }
