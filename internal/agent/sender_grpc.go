package agent

import (
	"context"
	"time"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	pb "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/proto"
	logger "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/pgk/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type GRPCSender struct {
	conn   *grpc.ClientConn
	client pb.MetricsClient
}

func NewGRPCSender(addr string) (*GRPCSender, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &GRPCSender{
		conn:   conn,
		client: pb.NewMetricsClient(conn),
	}, nil
}

func (s *GRPCSender) SendMetrics(ctx context.Context, metrics []model.Metrics) error {

	protoMetrics := make([]*pb.Metric, 0, len(metrics))

	for _, m := range metrics {
		pm := &pb.Metric{
			Id:   m.ID,
			Type: mapModelType(m.MType),
		}

		if m.Delta != nil {
			pm.Delta = *m.Delta
		}
		if m.Value != nil {
			pm.Value = *m.Value
		}

		protoMetrics = append(protoMetrics, pm)
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ip, err := getLocalIP()
	if err == nil {
		md := metadata.New(map[string]string{
			"x-real-ip": ip.String(),
		})
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	logger.NewHTTPLogger().Logger.Sugar().Infof(
		"gRPC agent sending metrics with x-real-ip=%s",
		ip,
	)

	_, err = s.client.UpdateMetrics(ctx, &pb.UpdateMetricsRequest{
		Metrics: protoMetrics,
	})

	return err
}

func (s *GRPCSender) Close() error {
	return s.conn.Close()
}

func mapModelType(t string) pb.Metric_MType {
	switch t {
	case model.Gauge:
		return pb.Metric_GAUGE
	case model.Counter:
		return pb.Metric_COUNTER
	default:
		return pb.Metric_GAUGE
	}
}

// проверяем на этапе компиляции
var _ model.MetricsSender = (*GRPCSender)(nil)

// для тестов
func NewGRPCSenderWithConn(conn *grpc.ClientConn) *GRPCSender {
	return &GRPCSender{
		conn:   conn,
		client: pb.NewMetricsClient(conn),
	}
}
