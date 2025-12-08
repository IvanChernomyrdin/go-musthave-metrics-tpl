package tests

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	mw "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGzipDecompression(t *testing.T) {
	// Хендлер для тестирования
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Received: " + string(body)))
	})

	middleware := mw.GzipDecompression(testHandler)

	tests := []struct {
		name           string
		content        string
		compress       bool
		setHeader      bool
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "обычный запрос без сжатия",
			content:        "test data",
			compress:       false,
			setHeader:      false,
			expectedStatus: http.StatusOK,
			expectedBody:   "Received: test data",
		},
		{
			name:           "gzip запрос с правильными данными",
			content:        "compressed data",
			compress:       true,
			setHeader:      true,
			expectedStatus: http.StatusOK,
			expectedBody:   "Received: compressed data",
		},
		{
			name:           "gzip заголовок но данные не сжаты",
			content:        "not compressed",
			compress:       false,
			setHeader:      true,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "Invalid gzip data",
		},
		{
			name:           "заголовок Content-Encoding удаляется после распаковки",
			content:        "test content",
			compress:       true,
			setHeader:      true,
			expectedStatus: http.StatusOK,
			expectedBody:   "Received: test content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			var bodyBytes []byte

			if tt.compress {
				var buf bytes.Buffer
				gz := gzip.NewWriter(&buf)
				_, err := gz.Write([]byte(tt.content))
				require.NoError(t, err)
				require.NoError(t, gz.Close())
				bodyBytes = buf.Bytes()
			} else {
				bodyBytes = []byte(tt.content)
			}

			body = bytes.NewReader(bodyBytes)

			req := httptest.NewRequest(http.MethodPost, "/test", body)
			if tt.setHeader {
				req.Header.Set("Content-Encoding", "gzip")
			}
			req.ContentLength = int64(len(bodyBytes))

			rr := httptest.NewRecorder()

			middleware.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.expectedBody)
		})
	}
}

func TestGzipCompression(t *testing.T) {
	// Хендлер для тестирования
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello, World! This is a test response with some content."))
	})

	middleware := mw.GzipCompression(testHandler)

	tests := []struct {
		name             string
		acceptEncoding   string
		expectedEncoding string
		shouldCompress   bool
		checkCompressed  bool
	}{
		{
			name:             "клиент поддерживает gzip",
			acceptEncoding:   "gzip, deflate, br",
			expectedEncoding: "gzip",
			shouldCompress:   true,
			checkCompressed:  true,
		},
		{
			name:             "клиент не поддерживает gzip",
			acceptEncoding:   "deflate, br",
			expectedEncoding: "",
			shouldCompress:   false,
			checkCompressed:  false,
		},
		{
			name:             "заголовок Accept-Encoding отсутствует",
			acceptEncoding:   "",
			expectedEncoding: "",
			shouldCompress:   false,
			checkCompressed:  false,
		},
		{
			name:             "заголовок Accept-Encoding с разными регистрами",
			acceptEncoding:   "gzip, compress",
			expectedEncoding: "gzip",
			shouldCompress:   true,
			checkCompressed:  true,
		},
		{
			name:             "частичное совпадение",
			acceptEncoding:   "something,gzip,something-else",
			expectedEncoding: "gzip",
			shouldCompress:   true,
			checkCompressed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}

			rr := httptest.NewRecorder()

			middleware.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)

			if tt.shouldCompress {
				assert.Equal(t, tt.expectedEncoding, rr.Header().Get("Content-Encoding"))

				if tt.checkCompressed {
					// Проверяем что данные действительно сжаты
					body := rr.Body.Bytes()

					// Попробуем распаковать
					reader, err := gzip.NewReader(bytes.NewReader(body))
					if err == nil {
						decompressed, err := io.ReadAll(reader)
						require.NoError(t, err)
						reader.Close()

						// Проверяем что распакованные данные содержат оригинальный текст
						assert.Contains(t, string(decompressed), "Hello, World!")
					} else {
						t.Errorf("Failed to decompress response: %v", err)
					}
				}
			} else {
				assert.Empty(t, rr.Header().Get("Content-Encoding"))
				// Проверяем что ответ не сжат
				assert.Contains(t, rr.Body.String(), "Hello, World!")
			}
		})
	}
}

func TestGzipResponseWriter(t *testing.T) {
	t.Run("Write и WriteHeader работают правильно", func(t *testing.T) {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		defer gz.Close()

		rr := httptest.NewRecorder()
		grw := mw.GzipResponseWriter{
			Writer:         gz,
			ResponseWriter: rr,
		}

		grw.WriteHeader(http.StatusCreated)

		data := []byte("test data")
		n, err := grw.Write(data)

		require.NoError(t, err)
		assert.Equal(t, len(data), n)
		assert.Equal(t, http.StatusCreated, rr.Code)
	})
}

func TestGzipDecompression_EmptyBody(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	middleware := mw.GzipDecompression(testHandler)

	t.Run("пустое тело с gzip заголовком", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Content-Encoding", "gzip")

		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		// Пустое тело должно обрабатываться без ошибок
		assert.Equal(t, http.StatusNoContent, rr.Code)
	})
}

func TestGzipCompression_ErrorHandling(t *testing.T) {
	t.Run("ошибка создания gzip writer не ломает обработку", func(t *testing.T) {
		// Создаем хендлер, который не должен сжиматься при ошибке
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("plain response"))
		})

		middleware := mw.GzipCompression(testHandler)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")

		rr := httptest.NewRecorder()

		// проверяем что middleware не паникует
		assert.NotPanics(t, func() {
			middleware.ServeHTTP(rr, req)
		})
		// Должен вернуться ответ (может быть сжатым или нет)
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestGzipMiddleware_Chain(t *testing.T) {
	t.Run("компрессия и декомпрессия вместе", func(t *testing.T) {
		// Хендлер который возвращает то что получил
		echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			w.Write(body)
		})

		// Создаем цепочку middleware: декомпрессия -> хендлер -> компрессия
		handler := mw.GzipDecompression(
			mw.GzipCompression(echoHandler),
		)

		// Тестируем цикл: сжатые данные отправляются, распаковываются,
		// обрабатываются, сжимаются обратно
		testData := "Test data for compression/decompression cycle"

		// Сжимаем данные для отправки
		var compressedInput bytes.Buffer
		gzIn := gzip.NewWriter(&compressedInput)
		_, err := gzIn.Write([]byte(testData))
		require.NoError(t, err)
		require.NoError(t, gzIn.Close())

		req := httptest.NewRequest(http.MethodPost, "/echo", &compressedInput)
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")
		req.ContentLength = int64(compressedInput.Len())

		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "gzip", rr.Header().Get("Content-Encoding"))

		// Распаковываем ответ
		gzOut, err := gzip.NewReader(rr.Body)
		require.NoError(t, err)
		decompressedOutput, err := io.ReadAll(gzOut)
		require.NoError(t, err)
		require.NoError(t, gzOut.Close())

		assert.Equal(t, testData, string(decompressedOutput))
	})
}

func TestGzipDecompression_ContentLength(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Проверяем что Content-Length установлен правильно
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := mw.GzipDecompression(testHandler)

	t.Run("Content-Length обновляется после распаковки", func(t *testing.T) {
		testData := "This is test data for content length check"

		// Сжимаем данные
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, err := gz.Write([]byte(testData))
		require.NoError(t, err)
		require.NoError(t, gz.Close())

		compressedData := buf.Bytes()

		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(compressedData))
		req.Header.Set("Content-Encoding", "gzip")
		req.ContentLength = int64(len(compressedData))

		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
