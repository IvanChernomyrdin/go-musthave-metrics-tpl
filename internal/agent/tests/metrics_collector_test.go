// Package tests
package tests

import (
	"testing"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsCollector(t *testing.T) {
	t.Run("collector creation", func(t *testing.T) {
		collector := agent.NewRuntimeMetricsCollector()
		require.NotNil(t, collector)
	})
	t.Run("collect metrics first time", func(t *testing.T) {
		collector := agent.NewRuntimeMetricsCollector()
		metrics := collector.Collect()

		require.NotEmpty(t, metrics)
		assert.GreaterOrEqual(t, len(metrics), 28)

		var hasPollCount, hasRandomValue, hasAlloc bool
		for _, metric := range metrics {
			switch metric.ID {
			case "PollCount":
				hasPollCount = true
				assert.Equal(t, "counter", metric.MType)
				assert.NotNil(t, metric.Delta)
				assert.Equal(t, int64(1), *metric.Delta)
			case "RandomValue":
				hasRandomValue = true
				assert.Equal(t, "gauge", metric.MType)
				assert.NotNil(t, metric.Value)
			case "Alloc":
				hasAlloc = true
				assert.Equal(t, "gauge", metric.MType)
				assert.NotNil(t, metric.Value)
			}
		}

		assert.True(t, hasPollCount, "PollCount metric should be present")
		assert.True(t, hasRandomValue, "RandomValue metric should be present")
		assert.True(t, hasAlloc, "Alloc metric should be present")
	})
	t.Run("poll count increments", func(t *testing.T) {
		collector := agent.NewRuntimeMetricsCollector()

		// Первый сбор
		metrics1 := collector.Collect()
		var pollCount1 int64
		for _, metric := range metrics1 {
			if metric.ID == "PollCount" {
				pollCount1 = *metric.Delta
				break
			}
		}
		assert.Equal(t, int64(1), pollCount1)

		// Второй сбор
		metrics2 := collector.Collect()
		var pollCount2 int64
		for _, metric := range metrics2 {
			if metric.ID == "PollCount" {
				pollCount2 = *metric.Delta
				break
			}
		}
		assert.Equal(t, int64(2), pollCount2)
	})

	t.Run("concurrent collection", func(t *testing.T) {
		collector := agent.NewRuntimeMetricsCollector()

		const goroutines = 10
		results := make(chan []string, goroutines)

		// Запускаем несколько горутин
		for i := 0; i < goroutines; i++ {
			go func() {
				metrics := collector.Collect()
				names := make([]string, 0, len(metrics))
				for _, m := range metrics {
					names = append(names, m.ID)
				}
				results <- names
			}()
		}

		// Собираем результаты
		metricSets := make([][]string, 0, goroutines)
		for i := 0; i < goroutines; i++ {
			metricSets = append(metricSets, <-results)
		}

		// Проверяем что все горутины вернули метрики
		for _, metrics := range metricSets {
			assert.NotEmpty(t, metrics)
			assert.Contains(t, metrics, "PollCount")
			assert.Contains(t, metrics, "RandomValue")
		}
	})

	t.Run("metrics structure", func(t *testing.T) {
		collector := agent.NewRuntimeMetricsCollector()
		metrics := collector.Collect()

		for _, metric := range metrics {
			assert.NotEmpty(t, metric.ID)
			assert.NotEmpty(t, metric.MType)

			switch metric.MType {
			case "gauge":
				assert.NotNil(t, metric.Value, "Gauge metric %s should have Value", metric.ID)
				assert.Nil(t, metric.Delta, "Gauge metric %s should not have Delta", metric.ID)
			case "counter":
				assert.NotNil(t, metric.Delta, "Counter metric %s should have Delta", metric.ID)
				assert.Nil(t, metric.Value, "Counter metric %s should not have Value", metric.ID)
			default:
				t.Errorf("Unknown metric type: %s for metric %s", metric.MType, metric.ID)
			}
		}
	})

	t.Run("random value range", func(t *testing.T) {
		collector := agent.NewRuntimeMetricsCollector()

		// Собираем несколько раз чтобы проверить разные значения
		values := make([]float64, 0, 5)
		for i := 0; i < 5; i++ {
			metrics := collector.Collect()
			for _, metric := range metrics {
				if metric.ID == "RandomValue" {
					values = append(values, *metric.Value)
					break
				}
			}
		}

		// 0 <= x < 1
		for _, value := range values {
			assert.GreaterOrEqual(t, value, 0.0)
			assert.Less(t, value, 1.0)
		}

		// Проверяем что значения разные (с большой вероятностью)
		uniqueValues := make(map[float64]bool)
		for _, value := range values {
			uniqueValues[value] = true
		}
		assert.Greater(t, len(uniqueValues), 1, "Random values should be different")
	})
}
