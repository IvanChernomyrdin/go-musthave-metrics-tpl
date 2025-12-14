// Package tests
package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/mocks"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	serv "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsService_UpdateGauge(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := serv.NewMetricsService(mockRepo)

	t.Run("успешное обновление gauge", func(t *testing.T) {
		mockRepo.On("UpsertGauge", "testGauge", 123.45).Return(nil).Once()

		err := service.UpdateGauge(context.Background(), "testGauge", 123.45)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("ошибка при обновлении gauge", func(t *testing.T) {
		expectedErr := fmt.Errorf("storage error")
		mockRepo.On("UpsertGauge", "testGauge", 123.45).Return(expectedErr).Once()

		err := service.UpdateGauge(context.Background(), "testGauge", 123.45)
		assert.Equal(t, expectedErr, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestMetricsService_UpdateCounter(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := serv.NewMetricsService(mockRepo)

	t.Run("успешное обновление counter", func(t *testing.T) {
		mockRepo.On("UpsertCounter", "testCounter", int64(100)).Return(nil).Once()

		err := service.UpdateCounter(context.Background(), "testCounter", 100)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("ошибка при обновлении counter", func(t *testing.T) {
		expectedErr := fmt.Errorf("storage error")
		mockRepo.On("UpsertCounter", "testCounter", int64(100)).Return(expectedErr).Once()

		err := service.UpdateCounter(context.Background(), "testCounter", 100)
		assert.Equal(t, expectedErr, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestMetricsService_GetValue(t *testing.T) {
	tests := []struct {
		name          string
		mtype         string
		metricName    string
		setupMock     func(*mocks.MetricsRepo)
		expectedVal   string
		expectedOK    bool
		expectedValid bool
	}{
		{
			name:       "получение существующего gauge",
			mtype:      serv.Gauge,
			metricName: "testGauge",
			setupMock: func(mockRepo *mocks.MetricsRepo) {
				mockRepo.On("GetGauge", "testGauge").Return(123.456, true).Once()
			},
			expectedVal:   "123.456",
			expectedOK:    true,
			expectedValid: true,
		},
		{
			name:       "получение gauge без десятичных знаков",
			mtype:      serv.Gauge,
			metricName: "testGauge",
			setupMock: func(mockRepo *mocks.MetricsRepo) {
				mockRepo.On("GetGauge", "testGauge").Return(123.0, true).Once()
			},
			expectedVal:   "123",
			expectedOK:    true,
			expectedValid: true,
		},
		{
			name:       "получение несуществующего gauge",
			mtype:      serv.Gauge,
			metricName: "nonexistent",
			setupMock: func(mockRepo *mocks.MetricsRepo) {
				mockRepo.On("GetGauge", "nonexistent").Return(0.0, false).Once()
			},
			expectedVal:   "",
			expectedOK:    false,
			expectedValid: true,
		},
		{
			name:       "получение существующего counter",
			mtype:      serv.Counter,
			metricName: "testCounter",
			setupMock: func(mockRepo *mocks.MetricsRepo) {
				mockRepo.On("GetCounter", "testCounter").Return(int64(100), true).Once()
			},
			expectedVal:   "100",
			expectedOK:    true,
			expectedValid: true,
		},
		{
			name:       "получение несуществующего counter",
			mtype:      serv.Counter,
			metricName: "nonexistent",
			setupMock: func(mockRepo *mocks.MetricsRepo) {
				mockRepo.On("GetCounter", "nonexistent").Return(int64(0), false).Once()
			},
			expectedVal:   "",
			expectedOK:    false,
			expectedValid: true,
		},
		{
			name:       "неверный тип метрики",
			mtype:      "invalid",
			metricName: "test",
			setupMock: func(mockRepo *mocks.MetricsRepo) {
				// Нет вызовов к репозиторию
			},
			expectedVal:   "",
			expectedOK:    false,
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MetricsRepo)
			service := serv.NewMetricsService(mockRepo)

			if tt.setupMock != nil {
				tt.setupMock(mockRepo)
			}

			val, found, valid := service.GetValue(context.Background(), tt.mtype, tt.metricName)
			assert.Equal(t, tt.expectedVal, val)
			assert.Equal(t, tt.expectedOK, found)
			assert.Equal(t, tt.expectedValid, valid)

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestMetricsService_AllText(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := serv.NewMetricsService(mockRepo)

	t.Run("получение всех метрик", func(t *testing.T) {
		gauges := map[string]float64{
			"gauge1": 123.45,
			"gauge2": 678.90,
		}
		counters := map[string]int64{
			"counter1": 100,
			"counter2": 200,
		}

		mockRepo.On("GetAll").Return(gauges, counters).Once()

		result := service.AllText(context.Background())

		expected := map[string]string{
			"gauge.gauge1":     "123.45",
			"gauge.gauge2":     "678.9",
			"counter.counter1": "100",
			"counter.counter2": "200",
		}

		assert.Equal(t, expected, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("пустые метрики", func(t *testing.T) {
		mockRepo.On("GetAll").Return(map[string]float64{}, map[string]int64{}).Once()

		result := service.AllText(context.Background())

		assert.Empty(t, result)
		mockRepo.AssertExpectations(t)
	})
}

func TestMetricsService_GetGauge(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := serv.NewMetricsService(mockRepo)

	t.Run("получение существующего gauge", func(t *testing.T) {
		mockRepo.On("GetGauge", "testGauge").Return(123.45, true).Once()

		val, found := service.GetGauge(context.Background(), "testGauge")
		assert.Equal(t, 123.45, val)
		assert.True(t, found)
		mockRepo.AssertExpectations(t)
	})

	t.Run("получение несуществующего gauge", func(t *testing.T) {
		mockRepo.On("GetGauge", "nonexistent").Return(0.0, false).Once()

		val, found := service.GetGauge(context.Background(), "nonexistent")
		assert.Equal(t, 0.0, val)
		assert.False(t, found)
		mockRepo.AssertExpectations(t)
	})
}

func TestMetricsService_GetCounter(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := serv.NewMetricsService(mockRepo)

	t.Run("получение существующего counter", func(t *testing.T) {
		mockRepo.On("GetCounter", "testCounter").Return(int64(100), true).Once()

		val, found := service.GetCounter(context.Background(), "testCounter")
		assert.Equal(t, int64(100), val)
		assert.True(t, found)
		mockRepo.AssertExpectations(t)
	})

	t.Run("получение несуществующего counter", func(t *testing.T) {
		mockRepo.On("GetCounter", "nonexistent").Return(int64(0), false).Once()

		val, found := service.GetCounter(context.Background(), "nonexistent")
		assert.Equal(t, int64(0), val)
		assert.False(t, found)
		mockRepo.AssertExpectations(t)
	})
}

func TestMetricsService_UpdateMetricsBatch(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := serv.NewMetricsService(mockRepo)

	t.Run("успешное обновление batch", func(t *testing.T) {
		metrics := []model.Metrics{
			{
				ID:    "gauge1",
				MType: serv.Gauge,
				Value: func() *float64 { v := 123.45; return &v }(),
			},
			{
				ID:    "counter1",
				MType: serv.Counter,
				Delta: func() *int64 { v := int64(100); return &v }(),
			},
		}

		mockRepo.On("UpdateMetricsBatch", metrics).Return(nil).Once()

		err := service.UpdateMetricsBatch(context.Background(), metrics)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("ошибка при обновлении batch", func(t *testing.T) {
		metrics := []model.Metrics{
			{
				ID:    "gauge1",
				MType: serv.Gauge,
				Value: func() *float64 { v := 123.45; return &v }(),
			},
		}

		expectedErr := fmt.Errorf("batch update error")
		mockRepo.On("UpdateMetricsBatch", metrics).Return(expectedErr).Once()

		err := service.UpdateMetricsBatch(context.Background(), metrics)
		assert.Equal(t, expectedErr, err)
		mockRepo.AssertExpectations(t)
	})
}

func TestMetricsService_SaveToFile(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := serv.NewMetricsService(mockRepo)

	t.Run("сохранение в файл", func(t *testing.T) {
		// Создаем временный файл
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "metrics.json")

		gauges := map[string]float64{
			"gauge1": 123.45,
		}
		counters := map[string]int64{
			"counter1": 100,
		}

		mockRepo.On("GetAll").Return(gauges, counters).Once()

		err := service.SaveToFile(context.Background(), filename)
		assert.NoError(t, err)

		// Проверяем что файл создан
		_, err = os.Stat(filename)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("пустой filename - пропускаем сохранение", func(t *testing.T) {
		err := service.SaveToFile(context.Background(), "")
		assert.NoError(t, err)
		// Не должно быть вызовов к репозиторию
	})

	t.Run("создание директории если не существует", func(t *testing.T) {
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "subdir", "metrics.json")

		mockRepo.On("GetAll").Return(map[string]float64{}, map[string]int64{}).Once()

		err := service.SaveToFile(context.Background(), filename)
		assert.NoError(t, err)

		// Проверяем что файл создан
		_, err = os.Stat(filename)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})
}

func TestMetricsService_LoadFromFile(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := serv.NewMetricsService(mockRepo)

	t.Run("загрузка из файла", func(t *testing.T) {
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "metrics.json")

		// Создаем тестовый файл
		metrics := []model.Metrics{
			{
				ID:    "gauge1",
				MType: serv.Gauge,
				Value: func() *float64 { v := 123.45; return &v }(),
			},
			{
				ID:    "counter1",
				MType: serv.Counter,
				Delta: func() *int64 { v := int64(100); return &v }(),
			},
		}

		file, err := os.Create(filename)
		require.NoError(t, err)
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(metrics)
		require.NoError(t, err)
		file.Close()

		// Настраиваем моки
		mockRepo.On("UpsertGauge", "gauge1", 123.45).Return(nil).Once()
		mockRepo.On("UpsertCounter", "counter1", int64(100)).Return(nil).Once()

		err = service.LoadFromFile(context.Background(), filename)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("файл не существует - пропускаем загрузку", func(t *testing.T) {
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "nonexistent.json")

		err := service.LoadFromFile(context.Background(), filename)
		assert.NoError(t, err)
		// Не должно быть вызовов к репозиторию
	})

	t.Run("пустой filename - пропускаем загрузку", func(t *testing.T) {
		err := service.LoadFromFile(context.Background(), "")
		assert.NoError(t, err)
		// Не должно быть вызовов к репозиторию
	})
}

func TestMetricsService_SaveOnUpdateMiddleware(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := serv.NewMetricsService(mockRepo)

	t.Run("middleware сохраняет после успешного POST /update", func(t *testing.T) {
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "metrics.json")

		middleware := service.SaveOnUpdateMiddleware(filename)

		// Мокаем сохранение в файл
		mockRepo.On("GetAll").Return(map[string]float64{}, map[string]int64{}).Once()

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodPost, "/update/gauge/test/123.45", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// Проверяем что файл создан
		_, err := os.Stat(filename)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})

	t.Run("middleware не сохраняет при ошибке", func(t *testing.T) {
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "metrics.json")

		middleware := service.SaveOnUpdateMiddleware(filename)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))

		req := httptest.NewRequest(http.MethodPost, "/update/gauge/test/123.45", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)

		// Файл не должен быть создан
		_, err := os.Stat(filename)
		assert.Error(t, err)
		// Не должно быть вызовов к репозиторию
	})

	t.Run("middleware не сохраняет для GET запросов", func(t *testing.T) {
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "metrics.json")

		middleware := service.SaveOnUpdateMiddleware(filename)

		handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest(http.MethodGet, "/update/gauge/test/123.45", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// Файл не должен быть создан
		_, err := os.Stat(filename)
		assert.Error(t, err)
		// Не должно быть вызовов к репозиторию
	})
}

func TestMetricsService_StartPeriodicSaving(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := serv.NewMetricsService(mockRepo)

	t.Run("периодическое сохранение", func(t *testing.T) {
		tmpDir := t.TempDir()
		filename := filepath.Join(tmpDir, "metrics.json")

		// Мокаем сохранение в файл
		mockRepo.On("GetAll").Return(map[string]float64{}, map[string]int64{}).Twice()

		ticker := service.StartPeriodicSaving(context.Background(), filename, 100*time.Millisecond)
		defer ticker.Stop()

		// Ждем немного чтобы тикер сработал
		time.Sleep(250 * time.Millisecond)

		// Проверяем что файл создан
		_, err := os.Stat(filename)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
	})
}
