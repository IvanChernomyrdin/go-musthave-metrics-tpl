package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/middleware"
	"github.com/stretchr/testify/assert"
)

func TestGetRealIPMiddleware_FromHeader(t *testing.T) {
	var gotIP string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = middleware.GetRealIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.GetRealIPMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "10.10.1.92")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "10.10.1.92", gotIP)
}

func TestGetRealIPMiddleware_FromRemoteAddr(t *testing.T) {
	var gotIP string

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIP = middleware.GetRealIPFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.GetRealIPMiddleware(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.10:54321"

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "192.168.1.10", gotIP)
}
