package memory

import (
	"testing"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemStorage_UpsertGauge(t *testing.T) {
	storage := New()

	t.Run("добавляет новый gauge", func(t *testing.T) {
		err := storage.UpsertGauge("temperature", 25.5)
		require.NoError(t, err)

		value, ok := storage.GetGauge("temperature")
		assert.True(t, ok)
		assert.Equal(t, 25.5, value)
	})

	t.Run("перезаписывает существующий gauge", func(t *testing.T) {
		err := storage.UpsertGauge("temperature", 30.0)
		require.NoError(t, err)

		value, ok := storage.GetGauge("temperature")
		assert.True(t, ok)
		assert.Equal(t, 30.0, value)
	})

	t.Run("возвращает false для несуществующего gauge", func(t *testing.T) {
		_, ok := storage.GetGauge("nonexistent")
		assert.False(t, ok)
	})
}

func TestMemStorage_UpsertCounter(t *testing.T) {
	storage := New()

	t.Run("добавляет новый counter", func(t *testing.T) {
		err := storage.UpsertCounter("requests", 1)
		require.NoError(t, err)

		value, ok := storage.GetCounter("requests")
		assert.True(t, ok)
		assert.Equal(t, int64(1), value)
	})

	t.Run("инкрементирует существующий counter", func(t *testing.T) {
		err := storage.UpsertCounter("requests", 2)
		require.NoError(t, err)

		value, ok := storage.GetCounter("requests")
		assert.True(t, ok)
		assert.Equal(t, int64(3), value) // 1 + 2 = 3
	})

	t.Run("работает с отрицательными значениями", func(t *testing.T) {
		err := storage.UpsertCounter("errors", -5)
		require.NoError(t, err)

		value, ok := storage.GetCounter("errors")
		assert.True(t, ok)
		assert.Equal(t, int64(-5), value)
	})
}

func TestMemStorage_GetAll(t *testing.T) {
	storage := New()

	storage.UpsertGauge("temperature", 25.5)
	storage.UpsertGauge("pressure", 1013.2)
	storage.UpsertCounter("requests", 10)
	storage.UpsertCounter("errors", 2)

	t.Run("возвращает все gauge и counter", func(t *testing.T) {
		gauges, counters := storage.GetAll()

		assert.Len(t, gauges, 2)
		assert.Equal(t, 25.5, gauges["temperature"])
		assert.Equal(t, 1013.2, gauges["pressure"])

		assert.Len(t, counters, 2)
		assert.Equal(t, int64(10), counters["requests"])
		assert.Equal(t, int64(2), counters["errors"])
	})

	t.Run("возвращает копии мап", func(t *testing.T) {
		gauges, counters := storage.GetAll()

		gauges["temperature"] = 99.9
		counters["requests"] = 999

		value, _ := storage.GetGauge("temperature")
		assert.Equal(t, 25.5, value) // оригинал не изменился

		value2, _ := storage.GetCounter("requests")
		assert.Equal(t, int64(10), value2) // оригинал не изменился
	})
}

func TestMemStorage_UpdateMetricsBatch(t *testing.T) {
	storage := New()

	metrics := []model.Metrics{
		{
			ID:    "temperature",
			MType: model.Gauge,
			Value: func() *float64 { v := 25.5; return &v }(),
		},
		{
			ID:    "pressure",
			MType: model.Gauge,
			Value: func() *float64 { v := 1013.2; return &v }(),
		},
		{
			ID:    "requests",
			MType: model.Counter,
			Delta: func() *int64 { v := int64(5); return &v }(),
		},
		{
			ID:    "requests",
			MType: model.Counter,
			Delta: func() *int64 { v := int64(3); return &v }(),
		},
	}

	t.Run("пакетное обновление работает корректно", func(t *testing.T) {
		err := storage.UpdateMetricsBatch(metrics)
		require.NoError(t, err)

		temp, ok := storage.GetGauge("temperature")
		assert.True(t, ok)
		assert.Equal(t, 25.5, temp)

		pressure, ok := storage.GetGauge("pressure")
		assert.True(t, ok)
		assert.Equal(t, 1013.2, pressure)

		requests, ok := storage.GetCounter("requests")
		assert.True(t, ok)
		assert.Equal(t, int64(8), requests) // 5 + 3 = 8
	})

	t.Run("игнорирует nil значения", func(t *testing.T) {
		metricsWithNil := []model.Metrics{
			{
				ID:    "test_gauge",
				MType: model.Gauge,
				Value: nil, // nil значение
			},
			{
				ID:    "test_counter",
				MType: model.Counter,
				Delta: nil, // nil значение
			},
		}

		err := storage.UpdateMetricsBatch(metricsWithNil)
		require.NoError(t, err)

		_, ok := storage.GetGauge("test_gauge")
		assert.False(t, ok)

		_, ok = storage.GetCounter("test_counter")
		assert.False(t, ok)
	})
}

func TestMemStorage_ConcurrentAccess(t *testing.T) {
	storage := New()

	const goroutines = 100
	const iterations = 1000

	t.Run("параллельный доступ к gauge", func(t *testing.T) {
		done := make(chan bool)

		for i := 0; i < goroutines; i++ {
			go func(id int) {
				for j := 0; j < iterations; j++ {
					value := float64(id*1000 + j)
					storage.UpsertGauge("concurrent_gauge", value)
					storage.GetGauge("concurrent_gauge")
				}
				done <- true
			}(i)
		}

		for i := 0; i < goroutines; i++ {
			<-done
		}

		// Проверяем что нет паники и можно получить значение
		value, ok := storage.GetGauge("concurrent_gauge")
		assert.True(t, ok)
		assert.NotZero(t, value)
	})

	t.Run("параллельный доступ к counter", func(t *testing.T) {
		done := make(chan bool)

		for i := 0; i < goroutines; i++ {
			go func() {
				for j := 0; j < iterations; j++ {
					storage.UpsertCounter("concurrent_counter", 1)
					storage.GetCounter("concurrent_counter")
				}
				done <- true
			}()
		}

		for i := 0; i < goroutines; i++ {
			<-done
		}

		value, ok := storage.GetCounter("concurrent_counter")
		assert.True(t, ok)
		assert.Equal(t, int64(goroutines*iterations), value)
	})
}

func TestMemStorage_Close(t *testing.T) {
	storage := New()

	t.Run("close не возвращает ошибку", func(t *testing.T) {
		err := storage.Close()
		assert.NoError(t, err)
	})

	t.Run("можно использовать после close", func(t *testing.T) {
		storage := New()
		storage.Close()

		err := storage.UpsertGauge("test", 1.0)
		assert.NoError(t, err)

		value, ok := storage.GetGauge("test")
		assert.True(t, ok)
		assert.Equal(t, 1.0, value)
	})
}

func TestMemStorage_New(t *testing.T) {
	t.Run("создает пустое хранилище", func(t *testing.T) {
		storage := New()

		gauges, counters := storage.GetAll()
		assert.Empty(t, gauges)
		assert.Empty(t, counters)
	})

	t.Run("разные экземпляры независимы", func(t *testing.T) {
		storage1 := New()
		storage2 := New()

		storage1.UpsertGauge("temp", 25.0)
		storage2.UpsertGauge("temp", 30.0)

		val1, _ := storage1.GetGauge("temp")
		val2, _ := storage2.GetGauge("temp")

		assert.Equal(t, 25.0, val1)
		assert.Equal(t, 30.0, val2)
	})
}
