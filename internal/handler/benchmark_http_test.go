// Package httpserver
package httpserver

import (
	"context"
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
		svc.UpdateGauge(context.Background(), "test", float64(i))
	}
}

func BenchmarkUpdateCounter(b *testing.B) {
	storage := memory.New()
	svc := service.NewMetricsService(storage)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.UpdateCounter(context.Background(), "test", int64(i))
	}
}

func BenchmarkUpdateMetricsBatch(b *testing.B) {
	storage := memory.New()
	svc := service.NewMetricsService(storage)

	metrics := []model.Metrics{
		{ID: "test1", MType: model.Gauge, Value: Ptr(1.5)},
		{ID: "test2", MType: model.Counter, Delta: Ptr(int64(10))},
		{ID: "test3", MType: model.Gauge, Value: Ptr(3.14)},
		{ID: "test4", MType: model.Counter, Delta: Ptr(int64(5))},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.UpdateMetricsBatch(context.Background(), metrics)
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
		svc.UpdateGauge(context.Background(), keys[i%1000], float64(i))
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
		svc.UpdateCounter(context.Background(), keys[i%1000], int64(i))
	}
}

func Ptr[T any](v T) *T {
	return &v
}
