package grpcserver

import (
	"log"
	"net"

	pb "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/proto"
	"google.golang.org/grpc"
)

type Server struct {
	grpcServer *grpc.Server
	addr       string
}

func New(addr string, metricsService pb.MetricsServer, trustedSubnet *net.IPNet) *Server {

	s := grpc.NewServer(
		grpc.UnaryInterceptor(
			SubnetInterceptor(trustedSubnet),
		),
	)

	pb.RegisterMetricsServer(s, metricsService)

	return &Server{
		grpcServer: s,
		addr:       addr,
	}
}

func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	log.Printf("gRPC сервер запущен на %s\n", s.addr)
	return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
}
