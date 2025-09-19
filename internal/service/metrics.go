package service

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	Counter = "counter"
	Gauge   = "gauge"
)

type MetricsRepo interface {
	UpsertGauge(id string, value float64) error
	UpsertCounter(id string, delta int64) error
	GetGauge(id string) (float64, bool)
	GetCounter(id string) (int64, bool)
	GetAll() (map[string]float64, map[string]int64)
}

type MetricsService struct {
	repo MetricsRepo
}

func NewMetricsService(repo MetricsRepo) *MetricsService {
	return &MetricsService{repo: repo}
}
func (ms *MetricsService) UpdateGauge(id string, value float64) error {
	return ms.repo.UpsertGauge(id, value)
}
func (ms *MetricsService) UpdateCounter(id string, delta int64) error {
	return ms.repo.UpsertCounter(id, delta)
}

func (ms *MetricsService) GetGauge(id string) (float64, bool) {
	return ms.repo.GetGauge(id)
}
func (ms *MetricsService) GetCounter(id string) (int64, bool) {
	return ms.repo.GetCounter(id)
}

func (s *MetricsService) GetValue(mtype, name string) (string, bool, bool) {
	switch mtype {
	case Gauge:
		if val, ok := s.repo.GetGauge(name); ok {
			out := strconv.FormatFloat(val, 'f', 3, 64)
			out = strings.TrimRight(out, "0")
			out = strings.TrimRight(out, ".")
			return out, true, true
		}
		return "", false, true
	case Counter:
		if val, ok := s.repo.GetCounter(name); ok {
			return strconv.FormatInt(val, 10), true, true
		}
		return "", false, true
	default:
		return "", false, false
	}
}

func (s *MetricsService) AllText() map[string]string {
	gs, cs := s.repo.GetAll()
	out := make(map[string]string, len(gs)+len(cs))

	for key, val := range gs {
		out[fmt.Sprintf("%s.%s", Gauge, key)] = strconv.FormatFloat(val, 'f', -1, 64)
	}
	for key, val := range cs {
		out[fmt.Sprintf("%s.%s", Counter, key)] = fmt.Sprintf("%d", val)
	}

	return out
}
