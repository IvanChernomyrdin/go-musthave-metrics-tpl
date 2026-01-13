package tests

import (
	"context"
	"net"
	"testing"

	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/agent"
	"github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/model"
	pb "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

type mockGRPCServer struct {
	pb.UnimplementedMetricsServer
	received []*pb.Metric
	md       metadata.MD
}

func (m *mockGRPCServer) UpdateMetrics(ctx context.Context, req *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {

	m.received = req.Metrics
	m.md, _ = metadata.FromIncomingContext(ctx)
	return &pb.UpdateMetricsResponse{}, nil
}

func setupBufConnServer(t *testing.T, srv pb.MetricsServer) (*grpc.ClientConn, func()) {
	t.Helper()

	lis := bufconn.Listen(bufSize)

	server := grpc.NewServer()
	pb.RegisterMetricsServer(server, srv)

	go func() {
		_ = server.Serve(lis)
	}()

	client, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient error: %v", err)
	}

	return client, func() {
		client.Close()
		server.Stop()
	}
}

func TestGRPCSender_SendMetrics_OK(t *testing.T) {
	mockSrv := &mockGRPCServer{}

	conn, cleanup := setupBufConnServer(t, mockSrv)
	defer cleanup()

	sender := agent.NewGRPCSenderWithConn(conn)

	val := 1.23
	delta := int64(5)

	err := sender.SendMetrics(context.Background(), []model.Metrics{
		{
			ID:    "cpu",
			MType: model.Gauge,
			Value: &val,
		},
		{
			ID:    "requests",
			MType: model.Counter,
			Delta: &delta,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mockSrv.received) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(mockSrv.received))
	}

	if mockSrv.received[0].Type != pb.Metric_GAUGE {
		t.Errorf("wrong gauge type")
	}

	if mockSrv.received[1].Type != pb.Metric_COUNTER {
		t.Errorf("wrong counter type")
	}

	if ips := mockSrv.md.Get("x-real-ip"); len(ips) == 0 {
		t.Errorf("x-real-ip metadata missing")
	}
}
