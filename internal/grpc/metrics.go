package grpcserver

import (
	"context"
	"log"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	pb "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/proto"
)

type MetricsUpdater interface {
	UpdateMetricsBatch(ctx context.Context, metrics []model.Metrics) error
}

type MetricsGRPCHandler struct {
	pb.UnimplementedMetricsServer
	svc MetricsUpdater
}

func NewMetricsHandler(svc MetricsUpdater) *MetricsGRPCHandler {
	return &MetricsGRPCHandler{svc: svc}
}

func (h *MetricsGRPCHandler) UpdateMetrics(ctx context.Context, req *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {

	log.Printf("gRPC UpdateMetrics called, metrics=%d", len(req.Metrics))

	metrics := make([]model.Metrics, 0, len(req.Metrics))

	for _, m := range req.Metrics {
		metric := model.Metrics{
			ID:    m.Id,
			MType: mapProtoType(m.Type),
		}

		switch m.Type {
		case pb.Metric_GAUGE:
			metric.Value = &m.Value
		case pb.Metric_COUNTER:
			metric.Delta = &m.Delta
		}

		metrics = append(metrics, metric)
	}

	err := h.svc.UpdateMetricsBatch(ctx, metrics)
	if err != nil {
		return nil, err
	}

	return &pb.UpdateMetricsResponse{}, nil
}

func mapProtoType(t pb.Metric_MType) string {
	switch t {
	case pb.Metric_GAUGE:
		return model.Gauge
	case pb.Metric_COUNTER:
		return model.Counter
	default:
		return ""
	}
}
