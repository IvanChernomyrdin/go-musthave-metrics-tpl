package httpserver

import (
	"net/http"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()

	// декомпрессия данных
	r.Use(middleware.GzipDecompression)
	// лоигрование
	r.Use(middleware.LoggerMiddleware())
	// компресия ответа
	r.Use(middleware.GzipCompression)

	r.Post("/value", h.GetValueJSON)
	r.Post("/value/", h.GetValueJSON)
	r.Post("/update", h.UpdateMetric)
	r.Post("/update/", h.UpdateMetric)
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
	r.Get("/value/{type}/{name}", h.GetValue)
	r.Get("/", h.GetAll)
	r.Get("/ping", h.PingDB)

	return r
}
