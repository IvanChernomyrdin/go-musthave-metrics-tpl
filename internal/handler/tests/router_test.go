package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	handlerhttp "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/handler"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/mocks"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

func setupTestRouter(handler *handlerhttp.Handler, HashKey string) *chi.Mux {
	r := chi.NewRouter()

	r.Post("/value", handler.GetValueJSON)
	r.Post("/value/", handler.GetValueJSON)
	r.Post("/update", handler.UpdateMetric)
	r.Post("/update/", handler.UpdateMetric)
	r.Post("/update/{type}/{name}/{value}", handler.UpdateMetric)
	r.Post("/updates/", handler.UpdateMetricsBatch)
	r.Get("/value/{type}/{name}", handler.GetValue)
	r.Get("/", handler.GetAll)
	r.Get("/ping", handler.PingDB)

	return r
}

func TestHandler_UpdateMetric_URLParams(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	svc := service.NewMetricsService(mockRepo)
	handler := handlerhttp.NewHandler(svc)
	router := setupTestRouter(handler, "")

	t.Run("обновление gauge через URL params", func(t *testing.T) {
		mockRepo.On("UpsertGauge", "testMetric", 123.45).Return(nil).Once()

		req := httptest.NewRequest(http.MethodPost, "/update/gauge/testMetric/123.45", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())
		mockRepo.AssertExpectations(t)
	})

	t.Run("обновление counter через URL params", func(t *testing.T) {
		mockRepo.On("UpsertCounter", "testCounter", int64(100)).Return(nil).Once()

		req := httptest.NewRequest(http.MethodPost, "/update/counter/testCounter/100", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())
		mockRepo.AssertExpectations(t)
	})
}

func TestHandler_UpdateMetric_JSON(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	svc := service.NewMetricsService(mockRepo)
	handler := handlerhttp.NewHandler(svc)
	router := setupTestRouter(handler, "")

	t.Run("обновление gauge через JSON", func(t *testing.T) {
		metric := model.Metrics{
			ID:    "testGauge",
			MType: "gauge",
			Value: func() *float64 { v := 123.45; return &v }(),
		}

		mockRepo.On("UpsertGauge", "testGauge", 123.45).Return(nil).Once()

		body, _ := json.Marshal(metric)
		req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "OK")
		mockRepo.AssertExpectations(t)
	})
}

func TestHandler_GetValue(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	svc := service.NewMetricsService(mockRepo)
	handler := handlerhttp.NewHandler(svc)
	router := setupTestRouter(handler, "")

	t.Run("получение gauge значения", func(t *testing.T) {
		mockRepo.On("GetGauge", "testMetric").Return(123.456, true).Once()

		req := httptest.NewRequest(http.MethodGet, "/value/gauge/testMetric", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "123.456", rr.Body.String())
		mockRepo.AssertExpectations(t)
	})

	t.Run("метрика не найдена", func(t *testing.T) {
		mockRepo.On("GetGauge", "nonexistent").Return(0.0, false).Once()

		req := httptest.NewRequest(http.MethodGet, "/value/gauge/nonexistent", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
		mockRepo.AssertExpectations(t)
	})
}

func TestHandler_GetAll(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	svc := service.NewMetricsService(mockRepo)
	handler := handlerhttp.NewHandler(svc)
	router := setupTestRouter(handler, "")

	t.Run("отображение всех метрик", func(t *testing.T) {
		gauges := map[string]float64{
			"gauge1": 123.45,
		}
		counters := map[string]int64{
			"counter1": 100,
		}

		mockRepo.On("GetAll").Return(gauges, counters).Once()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "gauge1")
		assert.Contains(t, rr.Body.String(), "counter1")
		assert.Contains(t, rr.Header().Get("Content-Type"), "text/html")
		mockRepo.AssertExpectations(t)
	})
}

func TestHandler_UpdateMetricsBatch(t *testing.T) {
	mockRepo := new(mocks.MetricsRepo)
	svc := service.NewMetricsService(mockRepo)
	handler := handlerhttp.NewHandler(svc)
	router := setupTestRouter(handler, "")

	t.Run("пакетное обновление метрик", func(t *testing.T) {
		metrics := []model.Metrics{
			{
				ID:    "gauge1",
				MType: "gauge",
				Value: func() *float64 { v := 123.45; return &v }(),
			},
			{
				ID:    "counter1",
				MType: "counter",
				Delta: func() *int64 { v := int64(100); return &v }(),
			},
		}

		mockRepo.On("UpsertGauge", "gauge1", 123.45).Return(nil).Once()
		mockRepo.On("UpsertCounter", "counter1", int64(100)).Return(nil).Once()

		body, _ := json.Marshal(metrics)
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "OK")
		mockRepo.AssertExpectations(t)
	})
}
