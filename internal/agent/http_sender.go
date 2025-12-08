package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	model "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	"github.com/go-resty/resty/v2"
	"golang.org/x/sync/errgroup"
)

type ErrorClassification int

const (
	NonRetriable ErrorClassification = iota
	Retriable
)

type HTTPErrorClassifier struct{}

func NewHTTPErrorClassifier() *HTTPErrorClassifier {
	return &HTTPErrorClassifier{}
}

func (c *HTTPErrorClassifier) ClassifyHTTPError(err error, statusCode int) ErrorClassification {
	if err == nil && statusCode == 0 {
		return NonRetriable
	}

	if err != nil && isRetriableNetworkError(err) {
		return Retriable
	}
	switch statusCode {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests:
		return Retriable
	default:
		if statusCode >= 500 {
			return Retriable
		}
		return NonRetriable
	}
}

type RetriableError struct {
	error
}

func (r RetriableError) Unwrap() error {
	return r.error
}

func NewRetriableError(err error) error {
	if err == nil {
		return nil
	}
	return RetriableError{err}
}

// проверяет, является ли ошибка повторяемой
func IsRetriableError(err error) bool {
	var retriableErr RetriableError
	return errors.As(err, &retriableErr)
}

type RetryConfig struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:  3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Second,
	}
}

// проверяет, является ли сетевая ошибка повторяемой
func isRetriableNetworkError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "network is unreachable") ||
		strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "temporary failure") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "connection closed")
}

type HTTPSender struct {
	client          *resty.Client
	url             string
	maxConc         int
	retryConfig     RetryConfig
	errorClassifier *HTTPErrorClassifier
	HashKey         string
}

func NewHTTPSender(serverURL string, HashKey string) *HTTPSender {
	client := resty.New()
	client.SetTimeout(10 * time.Second)

	return &HTTPSender{
		client:          client,
		url:             strings.TrimRight(serverURL, "/"),
		maxConc:         max(2, runtime.NumCPU()/2),
		retryConfig:     DefaultRetryConfig(),
		errorClassifier: NewHTTPErrorClassifier(),
		HashKey:         HashKey,
	}
}

func (s *HTTPSender) calculateHash256(b []byte) string {
	if s.HashKey == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(s.HashKey))
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *HTTPSender) Retry(ctx context.Context, operation func() error) error {
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}
	var lastErr error

	for attempt := 0; attempt < s.retryConfig.MaxAttempts; attempt++ {
		err := operation()
		if err == nil {
			return nil
		}
		lastErr = err

		//Проверяем, является ли ошибка повторяемой
		if !IsRetriableError(err) {
			return fmt.Errorf("неповторяемая ошибка: %w", err)
		}

		if attempt < len(delays) {
			delay := delays[attempt]
			select {
			case <-ctx.Done():
				return fmt.Errorf("операция отменена: %w", ctx.Err())
			case <-time.After(delay):
			}
		}
	}

	return fmt.Errorf("все %d попыток провалены, последняя ошибка: %w", s.retryConfig.MaxAttempts, lastErr)
}

func validateMetric(metric model.Metrics) error {
	if metric.ID == "" {
		return errors.New("empty ID metric")
	}
	switch metric.MType {
	case model.Counter:
		if metric.Delta == nil {
			return fmt.Errorf("metric %s has nil Delta", metric.ID)
		}
		if *metric.Delta < 0 {
			return fmt.Errorf("delta cannot be negative for metric %s", metric.ID)
		}
	case model.Gauge:
		if metric.Value == nil {
			return fmt.Errorf("metric %s has nil Value", metric.ID)
		}
	default:
		return fmt.Errorf("unknown metric type %s", metric.MType)
	}
	return nil
}

func (s *HTTPSender) SendMetrics(ctx context.Context, metrics []model.Metrics) error {
	validMetrics := make([]model.Metrics, 0, len(metrics))

	for _, metric := range metrics {
		if err := validateMetric(metric); err != nil {
			log.Printf("Invalid metric skipped: %v", err)
			continue
		}
		validMetrics = append(validMetrics, metric)
	}

	// Пробуем отправить батч с повторными попытками
	batchErr := s.Retry(ctx, func() error {
		return s.sendBatch(ctx, validMetrics)
	})

	if batchErr == nil {
		return nil
	}

	log.Printf("Batch sending failed, falling back to individual sends: %v", batchErr)

	// Ограничим параллелизм семафором
	semafor := make(chan struct{}, s.maxConc)
	g, gctx := errgroup.WithContext(ctx)

	for _, metric := range validMetrics {
		m := metric
		semafor <- struct{}{}

		g.Go(func() error {
			defer func() { <-semafor }()

			// Отправляем метрику с повторными попытками
			err := s.Retry(gctx, func() error {
				reqCtx, cancel := context.WithTimeout(gctx, 5*time.Second)
				defer cancel()
				return s.sendOne(reqCtx, m)
			})

			if err != nil {
				log.Printf("Failed to send metric %s after retries: %v", m.ID, err)
				// Не возвращаем ошибку, чтобы другие метрики могли отправиться
			}
			return nil
		})
	}

	return g.Wait()
}

func (s *HTTPSender) sendOne(ctx context.Context, metric model.Metrics) error {
	//сериализуем в json
	data, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}
	//сжимаем данные в gzip
	var compressionBuf bytes.Buffer
	gz := gzip.NewWriter(&compressionBuf)

	//записываем в gzip
	if _, err := gz.Write(data); err != nil {
		return fmt.Errorf("failed to write data to gzip: %w", err)
	}

	//принудительное закрытие gzip
	if err := gz.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Пробуем сначала новый JSON формат
	jsonErr := s.sendJSON(ctx, compressionBuf.Bytes())
	if jsonErr == nil {
		return nil
	}

	if IsRetriableError(jsonErr) {
		return jsonErr
	}

	// Если JSON не удался - пробуем старый формат
	return s.sendText(ctx, metric)
}

// Новый JSON формат
func (s *HTTPSender) sendJSON(ctx context.Context, metric []byte) error {
	base := strings.TrimRight(s.url, "/")
	fullURL := base + "/update/"

	req := s.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Content-Encoding", "gzip").
		SetBody(metric)

	if hash := s.calculateHash256(metric); hash != "" {
		req.SetHeader("HashSHA256", hash)
	}

	resp, err := req.Post(fullURL)

	if err != nil {
		// Классифицируем сетевую ошибку
		if s.errorClassifier.ClassifyHTTPError(err, 0) == Retriable {
			return NewRetriableError(fmt.Errorf("network error: %w", err))
		}
		return fmt.Errorf("request failed: %w", err)
	}

	// Классифицируем HTTP ошибку
	if s.errorClassifier.ClassifyHTTPError(nil, resp.StatusCode()) == Retriable {
		return NewRetriableError(fmt.Errorf("retriable status %d", resp.StatusCode()))
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("non-retriable status %d", resp.StatusCode())
	}

	return nil
}

// Старый text формат
func (s *HTTPSender) sendText(ctx context.Context, metric model.Metrics) error {
	idEscaped := url.PathEscape(metric.ID)

	var valueStr string
	switch metric.MType {
	case model.Counter:
		valueStr = strconv.FormatInt(*metric.Delta, 10)
	case model.Gauge:
		valueStr = strconv.FormatFloat(*metric.Value, 'f', -1, 64)
	default:
		return fmt.Errorf("unsupported metric type: %s", metric.MType)
	}

	base := strings.TrimRight(s.url, "/")
	fullURL := base + "/" + path.Join("update", metric.MType, idEscaped, valueStr)

	req := s.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "text/plain")

	// Для text формата нужно сериализовать данные для хеша
	textData := fmt.Sprintf("%s:%s:%s", metric.MType, metric.ID, valueStr)

	if hash := s.calculateHash256([]byte(textData)); hash != "" {
		req.SetHeader("HashSHA256", hash)
	}

	resp, err := req.Post(fullURL)

	if err != nil {
		if s.errorClassifier.ClassifyHTTPError(err, 0) == Retriable {
			return NewRetriableError(fmt.Errorf("network error: %w", err))
		}
		return fmt.Errorf("request failed: %w", err)
	}

	if s.errorClassifier.ClassifyHTTPError(nil, resp.StatusCode()) == Retriable {
		return NewRetriableError(fmt.Errorf("retriable status %d", resp.StatusCode()))
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("non-retriable status %d", resp.StatusCode())
	}

	return nil
}

// отправка батча
func (s *HTTPSender) sendBatch(ctx context.Context, metrics []model.Metrics) error {
	data, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal batch: %w", err)
	}

	var compressionBuf bytes.Buffer
	gz := gzip.NewWriter(&compressionBuf)
	if _, err := gz.Write(data); err != nil {
		return fmt.Errorf("failed to compress batch: %w", err)
	}
	if err := gz.Close(); err != nil {
		return fmt.Errorf("failed to close gzip: %w", err)
	}

	base := strings.TrimRight(s.url, "/")
	fullURL := base + "/updates/"

	req := s.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Content-Encoding", "gzip").
		SetBody(compressionBuf.Bytes())

	if hash := s.calculateHash256(compressionBuf.Bytes()); hash != "" {
		req.SetHeader("HashSHA256", hash)
	}

	resp, err := req.Post(fullURL)

	if err != nil {
		if s.errorClassifier.ClassifyHTTPError(err, 0) == Retriable {
			return NewRetriableError(fmt.Errorf("batch network error: %w", err))
		}
		return fmt.Errorf("batch request failed: %w", err)
	}

	if s.errorClassifier.ClassifyHTTPError(nil, resp.StatusCode()) == Retriable {
		return NewRetriableError(fmt.Errorf("batch retriable status %d", resp.StatusCode()))
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("batch non-retriable status %d", resp.StatusCode())
	}

	return nil
}
