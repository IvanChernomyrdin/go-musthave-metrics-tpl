package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
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

type HTTPSender struct {
	client  *resty.Client
	url     string
	maxConc int //кол-во горутин
}

func NewHTTPSender(serverURL string) *HTTPSender {
	client := resty.New()

	return &HTTPSender{
		client:  client,
		url:     strings.TrimRight(serverURL, "/"),
		maxConc: max(2, runtime.NumCPU()/2),
	}
}

func validateMetric(metric model.Metrics) error {
	if metric.ID == "" {
		log.Printf("empty ID metric")
	}
	switch metric.MType {
	case model.Counter:
		if metric.Delta == nil {
			log.Printf("metric %s has nil Delta", metric.ID)
		}
		if *metric.Delta < 0 {
			log.Printf("delta cannot be negative")
		}
	case model.Gauge:
		if metric.Value == nil {
			log.Printf("metric %s has nil Value", metric.ID)
		}
	default:
		log.Printf("unknown metric type %s", metric.MType)
	}
	return nil
}

func (s *HTTPSender) SendMetrics(ctx context.Context, metrics []model.Metrics) error {
	validMetrics := make([]model.Metrics, 0, len(metrics))

	for _, metric := range metrics {
		if err := validateMetric(metric); err != nil {
			continue
		}
		validMetrics = append(validMetrics, metric)
	}
	//ограничим параллелизм семафором
	semafor := make(chan struct{}, s.maxConc)

	g, gctx := errgroup.WithContext(ctx)

	for _, metric := range validMetrics {
		m := metric
		semafor <- struct{}{}
		g.Go(func() error {
			defer func() { <-semafor }()
			reqCtx, cancel := context.WithTimeout(gctx, 250*time.Millisecond)
			defer cancel()

			if err := s.sendOne(reqCtx, m); err != nil {
				log.Printf("send metric %s failed %v", m.ID, err)
				return nil
			}
			return nil
		})
	}
	_ = g.Wait()

	return nil
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
	if err := s.sendJSON(ctx, compressionBuf.Bytes()); err == nil {
		return nil
	}

	// Если JSON не удался - пробуем старый формат
	return s.sendText(ctx, metric)
}

// Новый JSON формат
func (s *HTTPSender) sendJSON(ctx context.Context, metric []byte) error {
	base := strings.TrimRight(s.url, "/")
	fullURL := base + "/update"

	resp, err := s.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetHeader("Content-Encoding", "gzip").
		SetBody(metric).
		Post(fullURL)

	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode())
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
		log.Printf("unsupported metric type: %s", metric.MType)
	}

	base := strings.TrimRight(s.url, "/")
	fullURL := base + "/" + path.Join("update", metric.MType, idEscaped, valueStr)

	resp, err := s.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "text/plain").
		Post(fullURL)

	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode())
	}

	return nil
}
