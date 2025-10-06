package httpserver

import (
	"net/http"
	"strings"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()

	// Убираем "/" в конце url
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" && strings.HasSuffix(r.URL.Path, "/") {
				r.URL.Path = strings.TrimSuffix(r.URL.Path, "/")
			}
			next.ServeHTTP(w, r)
		})
	})

	// декомпрессия данных
	r.Use(middleware.GzipDecompression)
	// лоигрование
	r.Use(middleware.LoggerMiddleware())
	// компресия ответа
	r.Use(middleware.GzipCompression)

	r.Post("/value", h.GetValueJSON)
	r.Post("/update", h.UpdateMetric)
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
	r.Get("/value/{type}/{name}", h.GetValue)
	r.Get("/", h.GetAll)

	return r
}
