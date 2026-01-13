package tests

import (
	"context"
	"testing"

	g "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/grpc"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	pb "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/proto"
)

type mockService struct {
	called  bool
	metrics []model.Metrics
	err     error
}

func (m *mockService) UpdateMetricsBatch(ctx context.Context, ms []model.Metrics) error {
	m.called = true
	m.metrics = ms
	return m.err
}

func TestUpdateMetrics(t *testing.T) {
	mockSvc := &mockService{}
	handler := g.NewMetricsHandler(mockSvc)

	req := &pb.UpdateMetricsRequest{
		Metrics: []*pb.Metric{
			{
				Id:    "cpu",
				Type:  pb.Metric_GAUGE,
				Value: 0.5,
			},
			{
				Id:    "requests",
				Type:  pb.Metric_COUNTER,
				Delta: 10,
			},
		},
	}

	_, err := handler.UpdateMetrics(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !mockSvc.called {
		t.Fatal("service was not called")
	}

	if len(mockSvc.metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(mockSvc.metrics))
	}

	if mockSvc.metrics[0].MType != model.Gauge {
		t.Errorf("wrong type for gauge")
	}

	if mockSvc.metrics[1].MType != model.Counter {
		t.Errorf("wrong type for counter")
	}
}
