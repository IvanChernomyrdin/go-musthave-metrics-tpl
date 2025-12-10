package tests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	middlwar "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errorReader struct{}

func TestNewHashMiddleware(t *testing.T) {
	t.Run("создает middleware с ключом", func(t *testing.T) {
		mw := middlwar.NewHashMiddleware("test-key")
		assert.NotNil(t, mw)
		assert.Equal(t, "test-key", mw.HashKey)
	})

	t.Run("создает middleware без ключа", func(t *testing.T) {
		mw := middlwar.NewHashMiddleware("")
		assert.NotNil(t, mw)
		assert.Equal(t, "", mw.HashKey)
	})
}

func TestHashMiddleware_ComputeHash(t *testing.T) {
	mw := middlwar.NewHashMiddleware("secret-key")

	tests := []struct {
		name     string
		data     string
		expected string
	}{
		{
			name:     "пустые данные",
			data:     "",
			expected: computeExpectedHash("secret-key", ""),
		},
		{
			name:     "обычные данные",
			data:     "test data",
			expected: computeExpectedHash("secret-key", "test data"),
		},
		{
			name:     "длинные данные",
			data:     strings.Repeat("a", 1000),
			expected: computeExpectedHash("secret-key", strings.Repeat("a", 1000)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := mw.ComputeHash([]byte(tt.data))
			assert.Equal(t, tt.expected, hash)
			assert.Equal(t, 64, len(hash)) // SHA256 hex длина
		})
	}
}

func TestHashMiddleware_CheckHash(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	tests := []struct {
		name           string
		HashKey        string
		method         string
		body           string
		headerHash     string
		setHeader      bool
		expectedStatus int
		shouldCall     bool
	}{
		{
			name:           "без ключа - пропускаем проверку",
			HashKey:        "",
			method:         http.MethodPost,
			body:           "test",
			headerHash:     "",
			setHeader:      false,
			expectedStatus: http.StatusOK,
			shouldCall:     true,
		},
		{
			name:           "GET запрос - пропускаем проверку",
			HashKey:        "secret",
			method:         http.MethodGet,
			body:           "test",
			headerHash:     "",
			setHeader:      false,
			expectedStatus: http.StatusOK,
			shouldCall:     true,
		},
		{
			name:           "правильный хэш",
			HashKey:        "secret",
			method:         http.MethodPost,
			body:           "test data",
			headerHash:     computeExpectedHash("secret", "test data"),
			setHeader:      true,
			expectedStatus: http.StatusOK,
			shouldCall:     true,
		},
		{
			name:           "неправильный хэш (логируется, но не блокируется)",
			HashKey:        "secret",
			method:         http.MethodPost,
			body:           "test data",
			headerHash:     "wrong-hash",
			setHeader:      true,
			expectedStatus: http.StatusOK, // только лог, не блокируем
			shouldCall:     true,
		},
		{
			name:           "отсутствует хэш в заголовке",
			HashKey:        "secret",
			method:         http.MethodPost,
			body:           "test data",
			headerHash:     "",
			setHeader:      false,
			expectedStatus: http.StatusOK,
			shouldCall:     true,
		},
		{
			name:           "PUT запрос с проверкой хэша",
			HashKey:        "secret",
			method:         http.MethodPut,
			body:           "update data",
			headerHash:     computeExpectedHash("secret", "update data"),
			setHeader:      true,
			expectedStatus: http.StatusOK,
			shouldCall:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := middlwar.NewHashMiddleware(tt.HashKey)
			handler := mw.CheckHash(testHandler)

			req := httptest.NewRequest(tt.method, "/test", bytes.NewReader([]byte(tt.body)))
			if tt.setHeader {
				req.Header.Set("HashSHA256", tt.headerHash)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.shouldCall {
				assert.Equal(t, "OK", rr.Body.String())
			}
		})
	}
}

func TestHashMiddleware_CheckHash_ErrorReadingBody(t *testing.T) {
	// Создаем специальный reader который вернет ошибку при чтении
	errorReader := &errorReader{}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("should not reach here"))
	})

	mw := middlwar.NewHashMiddleware("secret")
	handler := mw.CheckHash(testHandler)

	req := httptest.NewRequest(http.MethodPost, "/test", errorReader)
	req.Header.Set("HashSHA256", "some-hash")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Должна быть ошибка 400 при невозможности прочитать тело
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "Cannot read body")
}

func TestHashMiddleware_AddHash(t *testing.T) {
	tests := []struct {
		name           string
		hashKey        string
		responseBody   string
		expectedHeader bool
	}{
		{
			name:           "с ключом - добавляет хэш",
			hashKey:        "secret",
			responseBody:   "response data",
			expectedHeader: true,
		},
		{
			name:           "без ключа - не добавляет хэш",
			hashKey:        "",
			responseBody:   "response data",
			expectedHeader: false,
		},
		{
			name:           "пустой ответ - не добавляет хэш",
			hashKey:        "secret",
			responseBody:   "",
			expectedHeader: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Хендлер должен возвращать именно tt.responseBody
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				if tt.responseBody != "" {
					w.Write([]byte(tt.responseBody))
				}
			})

			mw := middlwar.NewHashMiddleware(tt.hashKey)
			handler := mw.AddHash(testHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, tt.responseBody, rr.Body.String())

			if tt.expectedHeader {
				expectedHash := computeExpectedHash(tt.hashKey, tt.responseBody)
				assert.Equal(t, expectedHash, rr.Header().Get("HashSHA256"))
			} else {
				assert.Empty(t, rr.Header().Get("HashSHA256"))
			}
		})
	}
}

func TestHashMiddleware_AddHash_WriteHeader(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	})

	mw := middlwar.NewHashMiddleware("secret")
	handler := mw.AddHash(testHandler)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "created", rr.Body.String())

	expectedHash := computeExpectedHash("secret", "created")
	assert.Equal(t, expectedHash, rr.Header().Get("HashSHA256"))
}

func TestHashMiddleware_AddHash_MultipleWrites(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("part1"))
		w.Write([]byte("part2"))
		w.Write([]byte("part3"))
	})

	mw := middlwar.NewHashMiddleware("secret")
	handler := mw.AddHash(testHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "part1part2part3", rr.Body.String())

	expectedHash := computeExpectedHash("secret", "part1part2part3")
	assert.Equal(t, expectedHash, rr.Header().Get("HashSHA256"))
}

func TestHashMiddleware_Chain(t *testing.T) {
	t.Run("цепочка CheckHash -> AddHash", func(t *testing.T) {
		// Хендлер который возвращает то что получил
		echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			w.WriteHeader(http.StatusOK)
			w.Write(body)
		})

		mw := middlwar.NewHashMiddleware("secret-key")

		// Создаем цепочку: проверка входящего хэша, обработка, добавление исходящего хэша
		handler := mw.CheckHash(
			mw.AddHash(echoHandler),
		)

		testData := "test message"
		correctHash := computeExpectedHash("secret-key", testData)

		req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte(testData)))
		req.Header.Set("HashSHA256", correctHash)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, testData, rr.Body.String())
		assert.Equal(t, correctHash, rr.Header().Get("HashSHA256"))
	})

	t.Run("цепочка без ключа", func(t *testing.T) {
		echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			w.Write(body)
		})

		mw := middlwar.NewHashMiddleware("") // пустой ключ
		handler := mw.CheckHash(mw.AddHash(echoHandler))

		testData := "test"
		req := httptest.NewRequest(http.MethodPost, "/echo", bytes.NewReader([]byte(testData)))
		req.Header.Set("HashSHA256", "some-hash") // должен игнорироваться

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, testData, rr.Body.String())
		assert.Empty(t, rr.Header().Get("HashSHA256")) // не должно добавляться
	})
}

func TestAddResponseWriter(t *testing.T) {
	t.Run("сохраняет тело ответа", func(t *testing.T) {
		originalWriter := httptest.NewRecorder()
		arw := &middlwar.AddResponseWriter{
			ResponseWriter: originalWriter,
		}

		data1 := []byte("hello")
		data2 := []byte(" world")

		n1, err1 := arw.Write(data1)
		n2, err2 := arw.Write(data2)

		assert.NoError(t, err1)
		assert.NoError(t, err2)

		assert.Equal(t, len(data1), n1)
		assert.Equal(t, len(data2), n2)
		assert.Equal(t, "hello world", string(arw.Body))
		assert.Equal(t, "hello world", originalWriter.Body.String())
	})

	t.Run("сохраняет статус код", func(t *testing.T) {
		originalWriter := httptest.NewRecorder()
		arw := &middlwar.AddResponseWriter{
			ResponseWriter: originalWriter,
		}

		arw.WriteHeader(http.StatusCreated)
		assert.Equal(t, http.StatusCreated, arw.Status)
		assert.Equal(t, http.StatusCreated, originalWriter.Code)
	})
}

func computeExpectedHash(key, data string) string {
	hmacHash := hmac.New(sha256.New, []byte(key))
	hmacHash.Write([]byte(data))
	return hex.EncodeToString(hmacHash.Sum(nil))
}
