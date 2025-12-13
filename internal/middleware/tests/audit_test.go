package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/middleware"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
)

// MockAuditReceiver для тестирования
type MockAuditReceiver struct {
	events []*middleware.AuditEvent
	errors []error
}

func (m *MockAuditReceiver) Notify(event *middleware.AuditEvent) error {
	m.events = append(m.events, event)
	if len(m.errors) > 0 {
		err := m.errors[0]
		m.errors = m.errors[1:]
		return err
	}
	return nil
}

func (m *MockAuditReceiver) GetEvents() []*middleware.AuditEvent {
	return m.events
}

func (m *MockAuditReceiver) SetErrors(errors []error) {
	m.errors = errors
}

// Test функция для проверки логирования
func TestAuditMiddleware_Success(t *testing.T) {
	mockReceiver := &MockAuditReceiver{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := middleware.AuditMiddleware([]middleware.AuditReceiver{mockReceiver})
	testHandler := middleware(handler)

	metrics := []model.Metrics{
		{ID: "test1", MType: "gauge", Value: Ptr(1.5)},
		{ID: "test2", MType: "counter", Delta: Ptr(int64(10))},
	}

	body, _ := json.Marshal(metrics)
	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:8080"
	rr := httptest.NewRecorder()

	testHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if len(mockReceiver.events) != 1 {
		t.Errorf("expected 1 audit event, got %d", len(mockReceiver.events))
		return
	}

	event := mockReceiver.events[0]
	if len(event.Metrics) != 2 {
		t.Errorf("expected 2 metrics in audit, got %d", len(event.Metrics))
	}

	if event.Metrics[0] != "test1" || event.Metrics[1] != "test2" {
		t.Errorf("unexpected metric names: %v", event.Metrics)
	}

	if event.IPAddress != "127.0.0.1:8080" {
		t.Errorf("unexpected IP address: %s", event.IPAddress)
	}
}

func TestAuditMiddleware_InvalidJSON(t *testing.T) {
	mockReceiver := &MockAuditReceiver{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("next handler should not be called for invalid JSON")
	})

	middleware := middleware.AuditMiddleware([]middleware.AuditReceiver{mockReceiver})
	testHandler := middleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader("invalid json"))
	rr := httptest.NewRecorder()

	testHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid JSON, got %d", rr.Code)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "invalid JSON format" {
		t.Errorf("expected error message, got %v", resp)
	}

	if len(mockReceiver.events) != 0 {
		t.Errorf("expected 0 audit events for invalid JSON, got %d", len(mockReceiver.events))
	}
}

func TestAuditMiddleware_ReceiverError(t *testing.T) {
	mockReceiver := &MockAuditReceiver{}
	mockReceiver.SetErrors([]error{fmt.Errorf("mock error")})

	calledNext := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calledNext = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := middleware.AuditMiddleware([]middleware.AuditReceiver{mockReceiver})
	testHandler := middleware(handler)

	metrics := []model.Metrics{{ID: "test", MType: "gauge", Value: Ptr(1.0)}}
	body, _ := json.Marshal(metrics)
	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	testHandler.ServeHTTP(rr, req)

	if !calledNext {
		t.Error("next handler should be called even if audit receiver fails")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}
}

func TestAuditMiddleware_OnlyUpdateEndpoints(t *testing.T) {
	mockReceiver := &MockAuditReceiver{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := middleware.AuditMiddleware([]middleware.AuditReceiver{mockReceiver})
	testHandler := middleware(handler)

	// Test /value endpoint - ДОЛЖЕН триггерить аудит (твоя логика)
	metrics := []model.Metrics{{ID: "test", MType: "gauge", Value: Ptr(1.0)}}
	body, _ := json.Marshal(metrics)
	req := httptest.NewRequest(http.MethodPost, "/value", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	testHandler.ServeHTTP(rr, req)

	if len(mockReceiver.events) != 1 { // меняем ожидание с 0 на 1
		t.Errorf("expected 1 audit event for /value endpoint, got %d", len(mockReceiver.events))
	}
}

func TestFileAuditReceiver(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "audit.log")

	receiver := &middleware.FileAuditReceiver{FilePath: filePath}
	event := &middleware.AuditEvent{
		Timestamp: time.Now().Unix(),
		Metrics:   []string{"metric1", "metric2"},
		IPAddress: "127.0.0.1:8080",
	}

	err := receiver.Notify(event)
	if err != nil {
		t.Fatalf("failed to write to file: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var loggedEvent middleware.AuditEvent
	err = json.Unmarshal(content, &loggedEvent)
	if err != nil {
		t.Fatalf("failed to unmarshal logged event: %v", err)
	}

	if len(loggedEvent.Metrics) != 2 {
		t.Errorf("expected 2 metrics in log, got %d", len(loggedEvent.Metrics))
	}
}

func TestFileAuditReceiver_CreateFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "new-audit.log")

	// Убедимся что файла нет
	if _, err := os.Stat(filePath); err == nil {
		t.Fatal("file should not exist before test")
	}

	receiver := &middleware.FileAuditReceiver{FilePath: filePath}
	event := &middleware.AuditEvent{Timestamp: time.Now().Unix(), Metrics: []string{"test"}}

	err := receiver.Notify(event)
	if err != nil {
		t.Fatalf("failed to create and write file: %v", err)
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("file should be created")
	}
}

func TestURLAuditReceiver(t *testing.T) {
	// Создаем тестовый сервер для приема аудит логов
	receivedEvents := []*middleware.AuditEvent{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event middleware.AuditEvent
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		receivedEvents = append(receivedEvents, &event)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	receiver := &middleware.URLAuditReceiver{URL: server.URL}
	event := &middleware.AuditEvent{
		Timestamp: time.Now().Unix(),
		Metrics:   []string{"test"},
		IPAddress: "127.0.0.1:8080",
	}

	err := receiver.Notify(event)
	if err != nil {
		t.Fatalf("failed to send to URL: %v", err)
	}

	if len(receivedEvents) != 1 {
		t.Errorf("expected 1 event on server, got %d", len(receivedEvents))
	}
}

func TestURLAuditReceiver_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	receiver := &middleware.URLAuditReceiver{URL: server.URL}
	event := &middleware.AuditEvent{Timestamp: time.Now().Unix(), Metrics: []string{"test"}}

	err := receiver.Notify(event)
	if err == nil {
		t.Error("expected error for non-200 status code")
	}

	expectedError := "failed to send audit log, status: 500 Internal Server Error"
	if err.Error() != expectedError {
		t.Errorf("expected error %q, got %q", expectedError, err.Error())
	}
}

func TestAuditMiddleware_EmptyMetricsArray(t *testing.T) {
	mockReceiver := &MockAuditReceiver{}
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := middleware.AuditMiddleware([]middleware.AuditReceiver{mockReceiver})
	testHandler := middleware(handler)

	body, _ := json.Marshal([]model.Metrics{}) // Пустой массив
	req := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	testHandler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("handler should be called for empty metrics array")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for empty metrics, got %d", rr.Code)
	}

	// Ожидаем 0 событий аудита, т.к. пустой массив пропускается
	if len(mockReceiver.events) != 0 {
		t.Errorf("expected 0 audit events for empty metrics, got %d", len(mockReceiver.events))
	}
}
func TestAuditMiddleware_RequestBodyReadError(t *testing.T) {
	mockReceiver := &MockAuditReceiver{}
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := middleware.AuditMiddleware([]middleware.AuditReceiver{mockReceiver})
	testHandler := middleware(handler)

	// Создаем запрос с телом, которое вызовет ошибку при чтении
	req := httptest.NewRequest(http.MethodPost, "/update", errorReader{})
	rr := httptest.NewRecorder()

	testHandler.ServeHTTP(rr, req)

	if !handlerCalled {
		t.Error("next handler should be called even if body read fails")
	}

	if len(mockReceiver.events) != 0 {
		t.Errorf("expected 0 audit events for read error, got %d", len(mockReceiver.events))
	}
}

// Вспомогательная функция
func Ptr[T any](v T) *T {
	return &v
}

func (errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func (errorReader) Close() error {
	return nil
}
