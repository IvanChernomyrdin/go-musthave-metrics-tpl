// Пакет handler_test содержит примеры использования HTTP-хендлеров метрик.
// Демонстрирует основные сценарии работы с API системы мониторинга.
package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	handlertest "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/handler"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/repository/memory"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/service"
	"github.com/go-chi/chi/v5"
)

// createTestServer создает тестовый HTTP сервер с настроенными маршрутами
func createTestServer(h *handlertest.Handler) *httptest.Server {
	r := chi.NewRouter()

	// Регистрируем маршруты как в основном приложении
	r.Post("/update", h.UpdateMetric)
	r.Post("/update/", h.UpdateMetric)
	r.Post("/update/{type}/{name}/{value}", h.UpdateMetric)
	r.Get("/value/{type}/{name}", h.GetValue)
	r.Get("/", h.GetAll)
	r.Post("/value", h.GetValueJSON)
	r.Get("/ping", h.PingDB)
	r.Post("/updates", h.UpdateMetricsBatch)

	return httptest.NewServer(r)
}

// Пример обновления метрики через URL параметры (старый формат).
// Демонстрирует использование эндпоинта /update/{type}/{name}/{value}.
func ExampleHandler_UpdateMetric_legacyFormat() {
	// Инициализация зависимостей
	storage := memory.New()
	svc := service.NewMetricsService(storage)
	h := handlertest.NewHandler(svc)

	// Создание тестового HTTP сервера
	server := createTestServer(h)
	defer server.Close()

	// Отправка запроса на обновление метрики типа gauge
	resp, err := http.Post(
		server.URL+"/update/gauge/Alloc/1234.56",
		"text/plain",
		nil,
	)
	if err != nil {
		fmt.Printf("Ошибка отправки запроса: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Проверка ответа
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n", strings.TrimSpace(string(body)))

	// Output:
	// Status: 200
	// Response: OK
}

// Пример обновления метрики через JSON (новый формат).
// Демонстрирует использование эндпоинта /update с телом запроса в формате JSON.
func ExampleHandler_UpdateMetric_jsonFormat() {
	storage := memory.New()
	svc := service.NewMetricsService(storage)
	h := handlertest.NewHandler(svc)

	server := createTestServer(h)
	defer server.Close()

	// Создание метрики в формате JSON
	metric := model.Metrics{
		ID:    "Alloc",
		MType: "gauge",
		Value: func() *float64 { v := 1234.56; return &v }(),
	}

	// Сериализация в JSON
	jsonData, _ := json.Marshal(metric)

	// Отправка POST запроса с JSON
	resp, err := http.Post(
		server.URL+"/update",
		"application/json",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		fmt.Printf("Ошибка отправки запроса: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Чтение ответа
	var response map[string]string
	json.NewDecoder(resp.Body).Decode(&response)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response status: %s\n", response["status"])

	// Output:
	// Status: 200
	// Response status: OK
}

// Пример получения значения метрики через URL.
// Демонстрирует использование эндпоинта /value/{type}/{name}.
func ExampleHandler_GetValue() {
	storage := memory.New()
	svc := service.NewMetricsService(storage)
	h := handlertest.NewHandler(svc)

	// Сначала добавляем метрику
	svc.UpdateGauge("Alloc", 1234.56)

	server := createTestServer(h)
	defer server.Close()

	// Получение значения метрики
	resp, err := http.Get(server.URL + "/value/gauge/Alloc")
	if err != nil {
		fmt.Printf("Ошибка получения метрики: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Value: %s\n", strings.TrimSpace(string(body)))

	// Output:
	// Status: 200
	// Value: 1234.56
}

// Пример получения метрики в JSON формате.
// Демонстрирует использование эндпоинта /value с телом запроса в формате JSON.
func ExampleHandler_GetValueJSON() {
	storage := memory.New()
	svc := service.NewMetricsService(storage)
	h := handlertest.NewHandler(svc)

	// Добавляем тестовую метрику
	svc.UpdateGauge("Alloc", 1234.56)

	server := createTestServer(h)
	defer server.Close()

	// Создание запроса для поиска метрики
	reqMetric := model.Metrics{
		ID:    "Alloc",
		MType: "gauge",
	}

	jsonData, _ := json.Marshal(reqMetric)

	// Отправка POST запроса
	resp, err := http.Post(
		server.URL+"/value",
		"application/json",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		fmt.Printf("Ошибка отправки запроса: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// Декодирование ответа
	var metric model.Metrics
	json.NewDecoder(resp.Body).Decode(&metric)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Found metric: ID=%s, Type=%s, Value=%.2f\n",
		metric.ID, metric.MType, *metric.Value)

	// Output:
	// Status: 200
	// Found metric: ID=Alloc, Type=gauge, Value=1234.56
}

// Пример пакетного обновления метрик.
// Демонстрирует использование эндпоинта /updates для массового обновления.
func ExampleHandler_UpdateMetricsBatch() {
	storage := memory.New()
	svc := service.NewMetricsService(storage)
	h := handlertest.NewHandler(svc)

	server := createTestServer(h)
	defer server.Close()

	// Создание массива метрик для пакетного обновления
	metrics := []model.Metrics{
		{
			ID:    "Alloc",
			MType: "gauge",
			Value: func() *float64 { v := 1234.56; return &v }(),
		},
		{
			ID:    "PollCount",
			MType: "counter",
			Delta: func() *int64 { d := int64(1); return &d }(),
		},
	}

	jsonData, _ := json.Marshal(metrics)

	// Отправка пакетного запроса
	resp, err := http.Post(
		server.URL+"/updates",
		"application/json",
		bytes.NewReader(jsonData),
	)
	if err != nil {
		fmt.Printf("Ошибка отправки запроса: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var response map[string]string
	json.NewDecoder(resp.Body).Decode(&response)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Batch update status: %s\n", response["status"])

	// Output:
	// Status: 200
	// Batch update status: OK
}

// Пример получения всех метрик в HTML формате.
// Демонстрирует использование корневого эндпоинта /.
func ExampleHandler_GetAll() {
	storage := memory.New()
	svc := service.NewMetricsService(storage)
	h := handlertest.NewHandler(svc)

	// Добавляем тестовые метрики
	svc.UpdateGauge("Alloc", 1234.56)
	svc.UpdateCounter("PollCount", 42)

	server := createTestServer(h)
	defer server.Close()

	// Запрос всех метрик
	resp, err := http.Get(server.URL + "/")
	if err != nil {
		fmt.Printf("Ошибка получения метрик: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	html := string(body)

	// Проверяем наличие ключевых элементов в HTML
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Content-Type: %s\n", resp.Header.Get("Content-Type"))
	fmt.Printf("Contains 'Metrics': %v\n", strings.Contains(html, "Metrics"))
	fmt.Printf("Contains 'Alloc': %v\n", strings.Contains(html, "Alloc"))
	fmt.Printf("Contains 'PollCount': %v\n", strings.Contains(html, "PollCount"))

	// Output:
	// Status: 200
	// Content-Type: text/html; charset=utf-8
	// Contains 'Metrics': true
	// Contains 'Alloc': true
	// Contains 'PollCount': true
}

// Пример обработки ошибки при неверном типе метрики.
func ExampleHandler_UpdateMetric_errorHandling() {
	storage := memory.New()
	svc := service.NewMetricsService(storage)
	h := handlertest.NewHandler(svc)

	server := createTestServer(h)
	defer server.Close()

	// Попытка обновить метрику с неверным типом
	resp, err := http.Post(
		server.URL+"/update/invalidType/TestMetric/123",
		"text/plain",
		nil,
	)
	if err != nil {
		fmt.Printf("Ошибка отправки запроса: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Error message: %s\n", strings.TrimSpace(string(body)))

	// Output:
	// Status: 400
	// Error message: unknown metric type: invalidtype
}

// Пример цепочки операций: добавление и получение метрики.
func ExampleHandler_workflow() {
	storage := memory.New()
	svc := service.NewMetricsService(storage)
	h := handlertest.NewHandler(svc)

	server := createTestServer(h)
	defer server.Close()

	// 1. Добавляем метрику через JSON
	metric := model.Metrics{
		ID:    "Temperature",
		MType: "gauge",
		Value: func() *float64 { v := 23.5; return &v }(),
	}

	jsonData, _ := json.Marshal(metric)
	resp, err := http.Post(server.URL+"/update", "application/json", bytes.NewReader(jsonData))
	if err != nil && resp != nil {
		resp.Body.Close()
	}
	// 2. Получаем её значение через URL
	resp, err = http.Get(server.URL + "/value/gauge/Temperature")
	if err != nil {
		resp.Body.Close()
		fmt.Printf("Error getting metric: %v\n", err)
	}

	body, _ := io.ReadAll(resp.Body)

	// 3. Получаем все метрики в HTML
	htmlResp, err := http.Get(server.URL + "/")
	if err != nil {
		fmt.Printf("Error getting all metrics in html: %v\n", err)
	}
	defer htmlResp.Body.Close()
	htmlBody, _ := io.ReadAll(htmlResp.Body)

	fmt.Printf("Metric value: %s\n", strings.TrimSpace(string(body)))
	fmt.Printf("HTML contains metric: %v\n", strings.Contains(string(htmlBody), "Temperature"))

	// Output:
	// Metric value: 23.5
	// HTML contains metric: true
}
