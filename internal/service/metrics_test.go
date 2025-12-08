package service

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/mocks"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
)

func TestNewMetricsService(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := NewMetricsService(mockRepo)

	assert.NotNil(t, service)
	assert.Equal(t, mockRepo, service.repo)
}

func TestMetricsService_UpdateGauge(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		value   float64
		wantErr bool
	}{
		{
			name:    "successful update",
			id:      "test_gauge",
			value:   123.45,
			wantErr: false,
		},
		{
			name:    "empty id",
			id:      "",
			value:   0.0,
			wantErr: false,
		},
		{
			name:    "negative value",
			id:      "negative_gauge",
			value:   -100.5,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MetricsRepo)
			service := NewMetricsService(mockRepo)

			mockRepo.On("UpsertGauge", tt.id, tt.value).Return(nil)

			err := service.UpdateGauge(tt.id, tt.value)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestMetricsService_UpdateCounter(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		delta   int64
		wantErr bool
	}{
		{
			name:    "positive delta",
			id:      "test_counter",
			delta:   10,
			wantErr: false,
		},
		{
			name:    "zero delta",
			id:      "zero_counter",
			delta:   0,
			wantErr: false,
		},
		{
			name:    "negative delta",
			id:      "negative_counter",
			delta:   -5,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MetricsRepo)
			service := NewMetricsService(mockRepo)

			mockRepo.On("UpsertCounter", tt.id, tt.delta).Return(nil)

			err := service.UpdateCounter(tt.id, tt.delta)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestMetricsService_GetGauge(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := NewMetricsService(mockRepo)

	expectedValue := 99.99
	mockRepo.On("GetGauge", "existing_gauge").Return(expectedValue, true)
	mockRepo.On("GetGauge", "non_existing_gauge").Return(0.0, false)

	t.Run("existing gauge", func(t *testing.T) {
		value, ok := service.GetGauge("existing_gauge")
		assert.True(t, ok)
		assert.Equal(t, expectedValue, value)
	})

	t.Run("non-existing gauge", func(t *testing.T) {
		value, ok := service.GetGauge("non_existing_gauge")
		assert.False(t, ok)
		assert.Equal(t, 0.0, value)
	})

	mockRepo.AssertExpectations(t)
}

func TestMetricsService_GetCounter(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := NewMetricsService(mockRepo)

	expectedValue := int64(42)
	mockRepo.On("GetCounter", "existing_counter").Return(expectedValue, true)
	mockRepo.On("GetCounter", "non_existing_counter").Return(int64(0), false)

	t.Run("existing counter", func(t *testing.T) {
		value, ok := service.GetCounter("existing_counter")
		assert.True(t, ok)
		assert.Equal(t, expectedValue, value)
	})

	t.Run("non-existing counter", func(t *testing.T) {
		value, ok := service.GetCounter("non_existing_counter")
		assert.False(t, ok)
		assert.Equal(t, int64(0), value)
	})

	mockRepo.AssertExpectations(t)
}

func TestMetricsService_GetValue(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := NewMetricsService(mockRepo)

	tests := []struct {
		name          string
		mtype         string
		metricName    string
		mockSetup     func()
		expected      string
		expectedOk    bool
		expectedValid bool
	}{
		{
			name:       "existing gauge",
			mtype:      Gauge,
			metricName: "cpu_usage",
			mockSetup: func() {
				mockRepo.On("GetGauge", "cpu_usage").Return(75.5, true)
			},
			expected:      "75.5",
			expectedOk:    true,
			expectedValid: true,
		},
		{
			name:       "gauge with trailing zeros",
			mtype:      Gauge,
			metricName: "memory_free",
			mockSetup: func() {
				mockRepo.On("GetGauge", "memory_free").Return(1024.000, true)
			},
			expected:      "1024",
			expectedOk:    true,
			expectedValid: true,
		},
		{
			name:       "non-existing gauge",
			mtype:      Gauge,
			metricName: "missing_gauge",
			mockSetup: func() {
				mockRepo.On("GetGauge", "missing_gauge").Return(0.0, false)
			},
			expected:      "",
			expectedOk:    false,
			expectedValid: true,
		},
		{
			name:       "existing counter",
			mtype:      Counter,
			metricName: "requests",
			mockSetup: func() {
				mockRepo.On("GetCounter", "requests").Return(int64(1000), true)
			},
			expected:      "1000",
			expectedOk:    true,
			expectedValid: true,
		},
		{
			name:       "non-existing counter",
			mtype:      Counter,
			metricName: "missing_counter",
			mockSetup: func() {
				mockRepo.On("GetCounter", "missing_counter").Return(int64(0), false)
			},
			expected:      "",
			expectedOk:    false,
			expectedValid: true,
		},
		{
			name:          "invalid metric type",
			mtype:         "invalid",
			metricName:    "some_metric",
			mockSetup:     func() {},
			expected:      "",
			expectedOk:    false,
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockSetup()

			result, ok, valid := service.GetValue(tt.mtype, tt.metricName)

			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.expectedOk, ok)
			assert.Equal(t, tt.expectedValid, valid)

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestMetricsService_AllText(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := NewMetricsService(mockRepo)

	gauges := map[string]float64{
		"cpu_usage":   75.5,
		"memory_free": 1024.0,
	}

	counters := map[string]int64{
		"requests": 1000,
		"errors":   5,
	}

	mockRepo.On("GetAll").Return(gauges, counters)

	result := service.AllText()

	expected := map[string]string{
		"gauge.cpu_usage":   "75.5",
		"gauge.memory_free": "1024",
		"counter.requests":  "1000",
		"counter.errors":    "5",
	}

	assert.Equal(t, expected, result)
	mockRepo.AssertExpectations(t)
}

func TestMetricsService_UpdateMetricsBatch(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := NewMetricsService(mockRepo)

	metrics := []model.Metrics{
		{
			ID:    "cpu_usage",
			MType: Gauge,
			Value: func() *float64 { v := 75.5; return &v }(),
		},
		{
			ID:    "requests",
			MType: Counter,
			Delta: func() *int64 { v := int64(10); return &v }(),
		},
	}

	mockRepo.On("UpdateMetricsBatch", metrics).Return(nil)

	err := service.UpdateMetricsBatch(metrics)
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
}

func TestMetricsService_SaveToFile(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		mockData  func() (map[string]float64, map[string]int64)
		setupMock func(*mocks.MetricsRepo)
		wantErr   bool
	}{
		{
			name:     "empty filename - no calls to repo",
			filename: "",
			mockData: func() (map[string]float64, map[string]int64) {
				return map[string]float64{}, map[string]int64{}
			},
			setupMock: func(mr *mocks.MetricsRepo) {
			},
			wantErr: false,
		},
		{
			name:     "with metrics data",
			filename: "test_metrics.json",
			mockData: func() (map[string]float64, map[string]int64) {
				return map[string]float64{"cpu": 50.5}, map[string]int64{"req": 100}
			},
			setupMock: func(mr *mocks.MetricsRepo) {
				gauges, counters := map[string]float64{"cpu": 50.5}, map[string]int64{"req": 100}
				mr.On("GetAll").Return(gauges, counters)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MetricsRepo)
			service := NewMetricsService(mockRepo)

			tt.setupMock(mockRepo)

			if tt.filename != "" {
				tempDir := t.TempDir()
				fullPath := filepath.Join(tempDir, tt.filename)
				err := service.SaveToFile(fullPath)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)

					_, err := os.Stat(fullPath)
					assert.NoError(t, err)
				}
			} else {
				err := service.SaveToFile(tt.filename)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestMetricsService_LoadFromFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		setup    func(*mocks.MetricsRepo)
		wantErr  bool
	}{
		{
			name:     "empty filename",
			filename: "",
			setup:    func(mr *mocks.MetricsRepo) {},
			wantErr:  false,
		},
		{
			name:     "file does not exist",
			filename: "/tmp/nonexistent.json",
			setup:    func(mr *mocks.MetricsRepo) {},
			wantErr:  false,
		},
		{
			name:     "successful load",
			filename: "/tmp/test_load.json",
			setup: func(mr *mocks.MetricsRepo) {
				file, _ := os.Create("/tmp/test_load.json")
				defer file.Close()
				metrics := []model.Metrics{
					{
						ID:    "cpu",
						MType: Gauge,
						Value: func() *float64 { v := 75.5; return &v }(),
					},
				}
				json.NewEncoder(file).Encode(metrics)

				mr.On("UpsertGauge", "cpu", 75.5).Return(nil)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(mocks.MetricsRepo)
			service := NewMetricsService(mockRepo)

			tt.setup(mockRepo)

			err := service.LoadFromFile(tt.filename)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.filename != "" && tt.filename != "/tmp/nonexistent.json" {
				os.Remove(tt.filename)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestMetricsService_StartPeriodicSaving(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	service := NewMetricsService(mockRepo)

	filename := "/tmp/periodic_test.json"
	interval := 100 * time.Millisecond

	// Настраиваем мок
	gauges := map[string]float64{"test": 1.0}
	counters := map[string]int64{"counter": 1}
	mockRepo.On("GetAll").Return(gauges, counters).Times(3)

	ticker := service.StartPeriodicSaving(filename, interval)
	defer ticker.Stop()

	time.Sleep(350 * time.Millisecond)

	_, err := os.Stat(filename)
	assert.NoError(t, err)

	os.Remove(filename)
	os.Remove(filename + ".tmp")

	mockRepo.AssertExpectations(t)
}

func TestResponseWriter(t *testing.T) {
	mockRW := &mocks.ResponseWriter{}

	rw := &ResponseWriter{
		ResponseWriter: mockRW,
		statusCode:     http.StatusOK,
	}

	mockRW.On("WriteHeader", http.StatusNotFound).Once()
	rw.WriteHeader(http.StatusNotFound)
	assert.Equal(t, http.StatusNotFound, rw.statusCode)

	mockRW.AssertExpectations(t)
}
