// Package httpserver
package httpserver

import (
	"net/http"

	_ "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/docs" //

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/middleware"
	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger"
)

func NewRouter(h *Handler, HashKey string, auditReceivers []middleware.AuditReceiver, privateKeyPath string) http.Handler {
	r := chi.NewRouter()

	// декомпрессия данных
	r.Use(middleware.GzipDecompression)
	// расшифровываем боди если был передан адрес на приватный ключ и если есть заголовок
	if privateKeyPath != "" {
		r.Use(middleware.DecryptMiddleware(privateKeyPath))
	}
	// лоигрование
	r.Use(middleware.LoggerMiddleware())
	// компресия ответа
	r.Use(middleware.GzipCompression)
	//аудит
	r.Use(middleware.AuditMiddleware(auditReceivers))

	//проверка и добавление хэша
	hashMiddleware := middleware.NewHashMiddleware(HashKey)
	r.Use(hashMiddleware.CheckHash)
	r.Use(hashMiddleware.AddHash)

	r.Post("/value", h.GetValueJSON)
	r.Post("/value/", h.GetValueJSON)
	r.Post("/update", h.UpdateMetric)
	r.Post("/update/", h.UpdateMetric)
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
	r.Post("/updates/", h.UpdateMetricsBatch)
	r.Get("/value/{type}/{name}", h.GetValue)
	r.Get("/", h.GetAll)
	r.Get("/ping", h.PingDB)

	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	return r
}
