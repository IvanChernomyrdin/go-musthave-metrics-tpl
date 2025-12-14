// Package httpserver
package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/config/db"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
	"github.com/go-chi/chi/v5"
)

// Handler обрабатывает HTTP запросы для работы с метриками.
// Содержит бизнес-логику сервиса через MetricsService.
type Handler struct {
	svc *service.MetricsService
}

func NewHandler(svc *service.MetricsService) *Handler { return &Handler{svc: svc} }

// UpdateMetric godoc
// @Tags Info
// @Summary Обновление метрики
// @Description Обновляет метрику. Поддерживает два формата:
//  1. JSON (новый) - отправка объекта Metrics в теле запроса
//  2. URL параметры (старый) - /update/{type}/{name}/{value}
//
// @Accept json
// @Accept plain
// @Produce json
// @Produce plain
// @Param metric body model.Metrics false "Метрика в JSON формате"
// @Param type path string true "Тип метрики" Enums(gauge, counter)
// @Param name path string true "Имя метрики"
// @Param value path string true "Значение метрики"
// @Success 200 {object} map[string]string "Объект со статусом OK"
// @Success 200 {string} string "Строка со статусом OK"
// @Failure 400 {string} string "Неверный запрос"
// @Failure 400 {string} string "Неверный запрос: bad gauge value, bad counter value, unknown metric type, metric ID is required, gauge value is required, counter delta is required"
// @Failure 404 {string} string "Not Found"
// @Failure 500 {string} string "Внутренняя ошибка сервера: store error"
// @Router /update [post]
// @Router /update/ [post]
// @Router /update/{type}/{name}/{value} [post]
func (h *Handler) UpdateMetric(w http.ResponseWriter, r *http.Request) {
	// НОВЫЙ ФОРМАТ JSON
	if r.Body != nil && r.ContentLength > 0 {
		var metric model.Metrics
		if err := json.NewDecoder(r.Body).Decode(&metric); err == nil {
			if err := h.processMetric(r.Context(), metric); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
			return
		}
		// Если не JSON - восстанавливаем body для старого формата
		if body, err := io.ReadAll(r.Body); err == nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
	}

	// СТАРЫЙ ФОРМАТ - text/plain
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	h.processURLParams(w, r)
}

func (h *Handler) processURLParams(w http.ResponseWriter, r *http.Request) {
	mType := strings.ToLower(chi.URLParam(r, "type"))
	id := chi.URLParam(r, "name")
	val := chi.URLParam(r, "value")

	if id == "" {
		http.NotFound(w, r)
		return
	}

	switch mType {
	case service.Gauge:
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			http.Error(w, "bad gauge value", http.StatusBadRequest)
			return
		}
		if err := h.svc.UpdateGauge(r.Context(), id, f); err != nil {
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}

	case service.Counter:
		d, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			http.Error(w, "bad counter value", http.StatusBadRequest)
			return
		}
		if err := h.svc.UpdateCounter(r.Context(), id, d); err != nil {
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}

	default:
		http.Error(w, fmt.Sprintf("unknown metric type: %s", mType), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (h *Handler) processMetric(ctx context.Context, metric model.Metrics) error {
	if metric.ID == "" {
		return fmt.Errorf("metric ID is required")
	}

	switch metric.MType {
	case service.Gauge:
		if metric.Value == nil {
			return fmt.Errorf("gauge value is required")
		}
		return h.svc.UpdateGauge(ctx, metric.ID, *metric.Value)

	case service.Counter:
		if metric.Delta == nil {
			return fmt.Errorf("counter delta is required")
		}
		return h.svc.UpdateCounter(ctx, metric.ID, *metric.Delta)

	default:
		return fmt.Errorf("unknown metric type: %s", metric.MType)
	}
}

// GetValue godoc
// @Tags Info
// @Summary Получение значения метрики
// @Description Получает значение метрики по её типу и имени.
//
//	Парсит параметры запроса, получая type и name,
//	ищет значение в соответствующем хранилище (counter или gauge).
//
// @Accept plain
// @Produce plain
// @Param type path string true "Тип метрики" Enums(gauge, counter)
// @Param name path string true "Имя метрики"
// @Success 200 {string} string "Значение метрики в виде строки"
// @Failure 400 {string} string "Неверный тип метрики"
// @Failure 404 {string} string "Метрика не найдена"
// @Router /value/{type}/{name} [get]
func (h *Handler) GetValue(w http.ResponseWriter, r *http.Request) {
	mtype := chi.URLParam(r, "type")
	name := chi.URLParam(r, "name")

	if name == "" {
		http.NotFound(w, r)
		return
	}

	val, found, typeOK := h.svc.GetValue(r.Context(), mtype, name)
	if !typeOK {
		http.Error(w, "bad metric type", http.StatusBadRequest)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(val))
}

// GetAll godoc
// @Tags Info
// @Summary Получение всех метрик в HTML формате
// @Description Возвращает HTML страницу со списком всех метрик (gauge и counter) из базы данных.
// @Produce html
// @Success 200 {string} string "HTML страница со списком метрик"
// @Failure 500 {string} string "Ошибка сервера"
// @Router / [get]
func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	all := h.svc.AllText(r.Context())

	const tpl = `<!doctype html>
	<html><head><meta charset="utf-8"><title>metrics</title></head>
	<body>
	<h1>Metrics</h1>
	<ul>
	{{- range $k, $v := . }}
	<li><b>{{$k}}</b>: {{$v}}</li>
	{{- end }}
	</ul>
	</body></html>`

	t := template.Must(template.New("idx").Parse(tpl))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = t.Execute(w, all)
}

// GetValueJSON godoc
// @Tags Info
// @Summary Получение метрики в JSON формате
// @Description Принимает JSON с полями id и type, возвращает структуру метрики с полями ID, Type, Value.
// @Accept json
// @Produce json
// @Param metric body model.Metrics true "Метрика для поиска (нужны только id и type)"
// @Success 200 {object} model.Metrics "Найденная метрика"
// @Failure 400 {object} map[string]string "Неверный формат JSON, отсутствуют обязательные поля, неизвестный тип метрики"
// @Failure 404 {object} map[string]string "Метрика не найдена"
// @Failure 500 {string} string "Внутренняя ошибка сервера"
// @Router /value [post]
func (h *Handler) GetValueJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Декодируем JSON запрос
	var reqMetric model.Metrics
	if err := json.NewDecoder(r.Body).Decode(&reqMetric); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON format"})
		return
	}

	// Валидация обязательных полей
	if reqMetric.ID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "metric ID is required"})
		return
	}
	if reqMetric.MType == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "metric type is required"})
		return
	}

	// То что будет возвращать, пока заполняем id и
	response := model.Metrics{
		ID:    reqMetric.ID,
		MType: reqMetric.MType,
	}

	// Получаем значение метрики из хранилища и добавляем в response
	switch reqMetric.MType {
	case service.Gauge:
		value, exists := h.svc.GetGauge(r.Context(), reqMetric.ID)
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "metric not found"})
			return
		}
		response.Value = &value

	case service.Counter:
		value, exists := h.svc.GetCounter(r.Context(), reqMetric.ID)
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "metric not found"})
			return
		}
		response.Delta = &value

	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "unknown metric type"})
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// PingDB godoc
// @Tags Info
// @Summary Проверка связи от базой данных(далее бд)
// @Description Проверяет доступность подключения к бд
// @Produce plain
// @Success 200 {string} string "ОК"
// @Failure 500 {string} string "Ошибка соединения с бд"
// @Router /ping [get]
func (h *Handler) PingDB(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		http.Error(w, "Ошибка соединения с базой данных", http.StatusInternalServerError)
		log.Printf("Ошибка при проверке соединения с БД: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// UpdateMetricsBatch godoc
// @Tags Info
// @Summary Пакетное обновление метрик
// @Description Принимает массив метрик в JSON формате и обновляет их все за один запрос
// @Accept json
// @Produce json
// @Param metrics body []model.Metrics true "Массив метрик для обновления"
// @Success 200 {object} map[string]string "Пример: {\"status\":\"OK\"}"
// @Failure 400 {object} map[string]string "Неверный JSON формат или пустой массив"
// @Failure 400 {object} map[string]interface{} "Пример: {\"error\":\"validation failed\",\"details\":[\"metric[0]: ID is required\"]}"
// @Failure 500 {object} map[string]string "Пример: {\"error\":\"failed to update metric Alloc\"}"
// @Router /updates [post]
func (h *Handler) UpdateMetricsBatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var metrics []model.Metrics
	if err := json.NewDecoder(r.Body).Decode(&metrics); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON format"})
		return
	}

	if len(metrics) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "empty batch"})
		return
	}

	var validationErrors []string
	for i, metric := range metrics {
		if metric.ID == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("metric[%d]: ID is required", i))
			continue
		}

		switch metric.MType {
		case service.Gauge:
			if metric.Value == nil {
				validationErrors = append(validationErrors, fmt.Sprintf("metric[%d]: gauge value is required", i))
			}
		case service.Counter:
			if metric.Delta == nil {
				validationErrors = append(validationErrors, fmt.Sprintf("metric[%d]: counter delta is required", i))
			}
		default:
			validationErrors = append(validationErrors, fmt.Sprintf("metric[%d]: unknown metric type: %s", i, metric.MType))
		}
	}

	if len(validationErrors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "validation failed",
			"details": validationErrors,
		})
		return
	}

	for _, metric := range metrics {
		if err := h.processMetric(r.Context(), metric); err != nil {
			log.Printf("Error updating metric %s: %v", metric.ID, err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("failed to update metric %s, err: %s", metric.ID, err)})
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "OK"})
}
