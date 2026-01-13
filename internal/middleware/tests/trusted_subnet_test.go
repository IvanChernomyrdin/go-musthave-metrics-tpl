package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/middleware"
	"github.com/stretchr/testify/assert"
)

func TestTrustedSubnetMiddleware_AllowedIP(t *testing.T) {
	called := false

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.TrustedSubnetMiddleware("192.168.1.0/24")
	handler := mw(next)

	req := httptest.NewRequest(http.MethodPost, "/update", nil)
	req = req.WithContext(
		context.WithValue(req.Context(), middleware.RealIPCtxKey{}, "192.168.1.10"),
	)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.True(t, called, "next handler should be called")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTrustedSubnetMiddleware_ForbiddenIP(t *testing.T) {
	called := false

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	mw := middleware.TrustedSubnetMiddleware("192.168.1.0/24")
	handler := mw(next)

	req := httptest.NewRequest(http.MethodPost, "/update", nil)
	req = req.WithContext(
		context.WithValue(req.Context(), middleware.RealIPCtxKey{}, "10.0.0.1"),
	)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.False(t, called, "next handler should NOT be called")
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestTrustedSubnetMiddleware_EmptySubnet(t *testing.T) {
	called := false

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	mw := middleware.TrustedSubnetMiddleware("")
	handler := mw(next)

	req := httptest.NewRequest(http.MethodPost, "/update", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, rr.Code)
}
