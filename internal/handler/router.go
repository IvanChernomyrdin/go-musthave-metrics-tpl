package httpserver

import (
	"net/http"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.LoggerMiddleware())

	//старый вариант
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
	//новый вариант
	r.Post("/update", h.UpdateMetric)

	//получение метрики по json
	r.Post("/value", h.GetValueJSON)

	r.Get("/value/{type}/{name}", h.GetValue)
	r.Get("/", h.GetAll)

	return r
}
