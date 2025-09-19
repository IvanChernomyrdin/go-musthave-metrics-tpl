package httpserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()

	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)

	r.Get("/value/{type}/{name}", h.GetValue)
	r.Get("/", h.GetAll)

	return r
}
