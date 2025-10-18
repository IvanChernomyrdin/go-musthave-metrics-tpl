package httpserver

import (
	"bytes"
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

type Handler struct {
	svc *service.MetricsService
}

func NewHandler(svc *service.MetricsService) *Handler { return &Handler{svc: svc} }

func (h *Handler) UpdateMetric(w http.ResponseWriter, r *http.Request) {
	// НОВЫЙ ФОРМАТ JSON
	if r.Body != nil && r.ContentLength > 0 {
		var metric model.Metrics
		if err := json.NewDecoder(r.Body).Decode(&metric); err == nil {
			if err := h.processMetric(metric); err != nil {
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
		if err := h.svc.UpdateGauge(id, f); err != nil {
			http.Error(w, "store error", http.StatusInternalServerError)
			return
		}

	case service.Counter:
		d, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			http.Error(w, "bad counter value", http.StatusBadRequest)
			return
		}
		if err := h.svc.UpdateCounter(id, d); err != nil {
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

func (h *Handler) processMetric(metric model.Metrics) error {
	if metric.ID == "" {
		return fmt.Errorf("metric ID is required")
	}

	switch metric.MType {
	case service.Gauge:
		if metric.Value == nil {
			return fmt.Errorf("gauge value is required")
		}
		return h.svc.UpdateGauge(metric.ID, *metric.Value)

	case service.Counter:
		if metric.Delta == nil {
			return fmt.Errorf("counter delta is required")
		}
		return h.svc.UpdateCounter(metric.ID, *metric.Delta)

	default:
		return fmt.Errorf("unknown metric type: %s", metric.MType)
	}
}

func (h *Handler) GetValue(w http.ResponseWriter, r *http.Request) {
	mtype := chi.URLParam(r, "type")
	name := chi.URLParam(r, "name")

	if name == "" {
		http.NotFound(w, r)
		return
	}

	val, found, typeOK := h.svc.GetValue(mtype, name)
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
	_, _ = w.Write([]byte(val))
}

func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	all := h.svc.AllText()

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
		value, exists := h.svc.GetGauge(reqMetric.ID)
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "metric not found"})
			return
		}
		response.Value = &value

	case service.Counter:
		value, exists := h.svc.GetCounter(reqMetric.ID)
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

func (h *Handler) PingDB(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		http.Error(w, "Ошибка соединения с базой данных", http.StatusInternalServerError)
		log.Printf("Ошибка при проверке соединения с БД: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
