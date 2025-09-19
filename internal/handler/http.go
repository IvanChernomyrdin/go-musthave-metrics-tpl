package httpserver

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc *service.MetricsService
}

func NewHandler(svc *service.MetricsService) *Handler { return &Handler{svc: svc} }

func (h *Handler) UpdateMetric(w http.ResponseWriter, r *http.Request) {
	mType := strings.ToLower(chi.URLParam(r, "type"))
	id := chi.URLParam(r, "name")
	val := chi.URLParam(r, "value")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

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
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))

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
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))

	default:
		http.Error(w, fmt.Sprintf("unknown metric type: %s", mType), http.StatusBadRequest)
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
