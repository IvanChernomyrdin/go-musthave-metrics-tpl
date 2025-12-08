package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/mocks"
)

func TestServerStartup(t *testing.T) {
	t.Run("test server creation", func(t *testing.T) {
		server := &http.Server{
			Addr:    ":0",
			Handler: http.NewServeMux(),
		}

		go func() {
			time.Sleep(100 * time.Millisecond)
			server.Shutdown(context.Background())
		}()

		err := server.ListenAndServe()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Server closed")
	})
}

func TestGracefulShutdown(t *testing.T) {
	t.Run("test shutdown with timeout", func(t *testing.T) {
		server := &http.Server{
			Addr:    ":0",
			Handler: http.NewServeMux(),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		go func() {
			time.Sleep(50 * time.Millisecond)
			server.Shutdown(ctx)
		}()

		err := server.ListenAndServe()
		require.Error(t, err)
		assert.Equal(t, http.ErrServerClosed, err)
	})

	t.Run("test server response", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		req := httptest.NewRequest("GET", "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())
	})
}

func TestFileOperations(t *testing.T) {
	t.Run("test file save and load", func(t *testing.T) {
		tempDir := t.TempDir()
		tempFile := tempDir + "/test.json"

		data := []byte(`{"test": "data"}`)
		err := os.WriteFile(tempFile, data, 0644)
		require.NoError(t, err)

		loadedData, err := os.ReadFile(tempFile)
		require.NoError(t, err)
		assert.Equal(t, data, loadedData)
	})

	t.Run("test non-existent file", func(t *testing.T) {
		_, err := os.Open("/non/existent/file.json")
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestStorageInterface(t *testing.T) {
	t.Run("mock storage methods", func(t *testing.T) {
		mockStorage := new(mocks.Storage)

		mockStorage.On("Close").Return(nil)

		mockStorage.On("GetGauge", "test").Return(1.0, true).Once()

		err := mockStorage.Close()
		assert.NoError(t, err)

		val, ok := mockStorage.GetGauge("test")
		assert.True(t, ok)
		assert.Equal(t, 1.0, val)

		mockStorage.AssertExpectations(t)
	})
}

func TestMainFunctionality(t *testing.T) {
	t.Run("test signal handling", func(t *testing.T) {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT)

		go func() {
			time.Sleep(10 * time.Millisecond)
			ch <- syscall.SIGINT
		}()

		select {
		case sig := <-ch:
			assert.Equal(t, syscall.SIGINT, sig)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for signal")
		}

		signal.Stop(ch)
	})

	t.Run("test context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()

		select {
		case <-ctx.Done():
			assert.Equal(t, context.Canceled, ctx.Err())
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timeout waiting for context cancellation")
		}
	})
}

func TestHTTPClient(t *testing.T) {
	t.Run("test successful http request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("pong"))
		}))
		defer server.Close()

		resp, err := http.Get(server.URL)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, "pong", string(body))
	})
}

func TestHTTPServerRoutes(t *testing.T) {
	t.Run("test 404 route", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})

		req := httptest.NewRequest("GET", "/nonexistent", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("test method not allowed", func(t *testing.T) {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("POST", "/", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
		assert.Contains(t, rr.Body.String(), "Method not allowed")
	})
}

func TestTemporaryFiles(t *testing.T) {
	t.Run("create and delete temp file", func(t *testing.T) {
		tempDir := t.TempDir()
		tempFile, err := os.CreateTemp(tempDir, "test-*.txt")
		require.NoError(t, err)
		defer tempFile.Close()

		_, err = tempFile.WriteString("test content")
		require.NoError(t, err)

		fileInfo, err := os.Stat(tempFile.Name())
		require.NoError(t, err)
		assert.False(t, fileInfo.IsDir())
		assert.Greater(t, fileInfo.Size(), int64(0))
	})
}
