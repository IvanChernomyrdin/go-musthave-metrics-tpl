package tests

import (
	"context"
	"net"
	"testing"

	g "github.com/IvanChernomyrdin/go-musthave-metrics-tpl/internal/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestSubnetInterceptor(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("127.0.0.0/8")

	interceptor := g.SubnetInterceptor(subnet)

	handlerCalled := false
	handler := func(ctx context.Context, req any) (any, error) {
		handlerCalled = true
		return "ok", nil
	}

	t.Run("allowed ip", func(t *testing.T) {
		handlerCalled = false

		md := metadata.New(map[string]string{
			"x-real-ip": "127.0.0.1",
		})
		ctx := metadata.NewIncomingContext(context.Background(), md)

		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !handlerCalled {
			t.Fatal("handler was not called")
		}
	})

	t.Run("ip not allowed", func(t *testing.T) {
		md := metadata.New(map[string]string{
			"x-real-ip": "10.0.0.1",
		})
		ctx := metadata.NewIncomingContext(context.Background(), md)

		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("no metadata", func(t *testing.T) {
		_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("expected PermissionDenied, got %v", err)
		}
	})

	t.Run("no ip", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.New(nil))
		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
		if status.Code(err) != codes.PermissionDenied {
			t.Fatalf("expected PermissionDenied, got %v", err)
		}
	})
}
