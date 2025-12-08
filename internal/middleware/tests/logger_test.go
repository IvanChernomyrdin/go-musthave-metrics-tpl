package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockHTTPLogger для тестирования
type MockHTTPLogger struct {
	mock.Mock
}

func (m *MockHTTPLogger) LogRequest(method, uri string, status, size int, duration float64) {
	m.Called(method, uri, status, size, duration)
}

func createTestLoggerMiddleware(mockLogger *MockHTTPLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wr := &middleware.ResponseWriter{ResponseWriter: w}
			next.ServeHTTP(wr, r)

			duration := time.Since(start).Seconds() * 1000
			mockLogger.LogRequest(r.Method, r.RequestURI, wr.Status, wr.Size, duration)
		})
	}
}

func TestResponseWriter(t *testing.T) {
	t.Run("WriteHeader устанавливает статус", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := &middleware.ResponseWriter{ResponseWriter: rr}

		rw.WriteHeader(http.StatusCreated)
		assert.Equal(t, http.StatusCreated, rw.Status)
		assert.Equal(t, http.StatusCreated, rr.Code)
	})

	t.Run("Write устанавливает статус 200 если не установлен", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := &middleware.ResponseWriter{ResponseWriter: rr}

		data := []byte("test")
		n, err := rw.Write(data)

		assert.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, http.StatusOK, rw.Status)
		assert.Equal(t, data, rr.Body.Bytes())
	})

	t.Run("Write суммирует размер записанных данных", func(t *testing.T) {
		rr := httptest.NewRecorder()
		rw := &middleware.ResponseWriter{ResponseWriter: rr}

		data1 := []byte("hello")
		data2 := []byte(" world")

		n1, err1 := rw.Write(data1)
		n2, err2 := rw.Write(data2)

		assert.NoError(t, err1)
		assert.NoError(t, err2)
		assert.Equal(t, len(data1), n1)
		assert.Equal(t, len(data2), n2)
		assert.Equal(t, len(data1)+len(data2), rw.Size)
		assert.Equal(t, "hello world", rr.Body.String())
	})
}

func TestLoggerMiddleware(t *testing.T) {
	t.Run("логирует успешный запрос", func(t *testing.T) {
		mockLogger := new(MockHTTPLogger)

		mockLogger.On("LogRequest", "GET", "/test", http.StatusOK, 11, mock.AnythingOfType("float64")).Once()

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello World"))
		})

		middleware := createTestLoggerMiddleware(mockLogger)
		handler := middleware(testHandler)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Hello World", rr.Body.String())
		mockLogger.AssertExpectations(t)
	})

	t.Run("логирует запрос с ошибкой", func(t *testing.T) {
		mockLogger := new(MockHTTPLogger)

		mockLogger.On("LogRequest", "POST", "/api", http.StatusBadRequest, 11, mock.AnythingOfType("float64")).Once()

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Bad Request"))
		})

		middleware := createTestLoggerMiddleware(mockLogger)
		handler := middleware(testHandler)

		req := httptest.NewRequest("POST", "/api", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Equal(t, "Bad Request", rr.Body.String())
		mockLogger.AssertExpectations(t)
	})

	t.Run("логирует запрос без тела", func(t *testing.T) {
		mockLogger := new(MockHTTPLogger)

		mockLogger.On("LogRequest", "DELETE", "/item/123", http.StatusNoContent, 0, mock.AnythingOfType("float64")).Once()

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})

		middleware := createTestLoggerMiddleware(mockLogger)
		handler := middleware(testHandler)

		req := httptest.NewRequest("DELETE", "/item/123", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNoContent, rr.Code)
		assert.Empty(t, rr.Body.String())
		mockLogger.AssertExpectations(t)
	})

	t.Run("измеряет время выполнения", func(t *testing.T) {
		mockLogger := new(MockHTTPLogger)

		mockLogger.On("LogRequest", "GET", "/slow", http.StatusOK, 0, mock.AnythingOfType("float64")).
			Run(func(args mock.Arguments) {
				duration := args.Get(4).(float64)
				assert.Greater(t, duration, 0.0)
				assert.Greater(t, duration, 80.0)
				assert.Less(t, duration, 120.0)
			}).Once()

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// slow запрос
			time.Sleep(100 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		})

		middleware := createTestLoggerMiddleware(mockLogger)
		handler := middleware(testHandler)

		req := httptest.NewRequest("GET", "/slow", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		mockLogger.AssertExpectations(t)
	})

	t.Run("логирует размер ответа с несколькими Write", func(t *testing.T) {
		mockLogger := new(MockHTTPLogger)

		mockLogger.On("LogRequest", "GET", "/chunked", http.StatusOK, 12, mock.AnythingOfType("float64")).Once()

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("Hello"))
			w.Write([]byte(" "))
			w.Write([]byte("World"))
			w.Write([]byte("!"))
		})

		middleware := createTestLoggerMiddleware(mockLogger)
		handler := middleware(testHandler)

		req := httptest.NewRequest("GET", "/chunked", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "Hello World!", rr.Body.String())
		mockLogger.AssertExpectations(t)
	})
}

func TestRealLoggerMiddleware(t *testing.T) {
	t.Run("реальный middleware работает", func(t *testing.T) {
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("OK"))
		})

		middleware := middleware.LoggerMiddleware()
		handler := middleware(testHandler)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		assert.NotPanics(t, func() {
			handler.ServeHTTP(rr, req)
		})

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())
	})
}
