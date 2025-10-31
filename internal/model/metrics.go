package model

import (
	"context"
	"time"
)

const (
	Counter = "counter"
	Gauge   = "gauge"
)

type Metrics struct {
	ID    string   `json:"id"`
	MType string   `json:"type"`
	Delta *int64   `json:"delta,omitempty"`
	Value *float64 `json:"value,omitempty"`
	Hash  string   `json:"hash,omitempty"`
}

type MetricsCollector interface {
	Collect() []Metrics
	CollectSystemMetrics() []Metrics
}

type MetricsSender interface {
	SendMetrics(ctx context.Context, metrics []Metrics) error
}

type ConfigProvider interface {
	GetServerURL() string
	GetPollInterval() time.Duration
	GetReportInterval() time.Duration
	GetHash() string
	GetRateLimit() int
}
