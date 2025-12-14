// Package middleware
package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/runtime"
)

type AuditEvent struct {
	Timestamp int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

type AuditReceiver interface {
	Notify(event *AuditEvent) error
}

type FileAuditReceiver struct {
	FilePath string
}

func (f *FileAuditReceiver) Notify(event *AuditEvent) error {
	file, err := os.OpenFile(f.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = file.Write(append(data, '\n'))
	return err
}

type URLAuditReceiver struct {
	URL string
}

func (u *URLAuditReceiver) Notify(event *AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	resp, err := http.Post(u.URL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to send audit log, status: %s", resp.Status)
	}
	return nil
}

// AuditMiddleware - извлекает метрики и передает их в аудит
func AuditMiddleware(auditReceivers []AuditReceiver) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			if r.URL.Path != "/update" &&
				r.URL.Path != "/update/" &&
				r.URL.Path != "/updates" &&
				r.URL.Path != "/updates/" &&
				r.URL.Path != "/value" {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				runtime.NewHTTPLogger().Logger.Sugar().Warnf("Error reading request body: %v", err)
				next.ServeHTTP(w, r)
				return
			}

			r.Body = io.NopCloser(bytes.NewBuffer(body))

			var metrics []model.Metrics
			if err := json.NewDecoder(r.Body).Decode(&metrics); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON format"})
				return
			}

			runtime.NewHTTPLogger().Logger.Sugar().Infof("Extracted metrics: %v", metrics)

			var auditMetrics []string
			for _, metric := range metrics {
				auditMetrics = append(auditMetrics, metric.ID)
			}

			if len(auditMetrics) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			event := &AuditEvent{
				Timestamp: time.Now().Unix(),
				Metrics:   auditMetrics,
				IPAddress: r.RemoteAddr,
			}

			for _, receiver := range auditReceivers {
				if err := receiver.Notify(event); err != nil {
					runtime.NewHTTPLogger().Logger.Sugar().Warnf("Error while sending audit: %v", err)
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
