package middleware

import (
	"net/http"
	"time"

	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/runtime"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	size, err := w.ResponseWriter.Write(b)
	w.size += size
	return size, err
}

func LoggerMiddleware() func(http.Handler) http.Handler {
	log := logger.NewHTTPLogger()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wr := &responseWriter{ResponseWriter: w}
			next.ServeHTTP(wr, r)

			duration := time.Since(start).Seconds() * 1000
			log.LogRequest(r.Method, r.RequestURI, wr.status, wr.size, duration)
		})
	}
}
