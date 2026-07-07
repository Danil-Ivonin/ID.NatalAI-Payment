package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Danil-Ivonin/ID.NatalAI-Payment/gen/payment"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/adapters/grpc"
	"github.com/Danil-Ivonin/ID.NatalAI-Payment/internal/app/usecase"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	defaultGRPCAddr         = ":50051"
	grpcShutdownGracePeriod = 15 * time.Second
)

func main() {
	if err := run(context.Background()); err != nil {
		slog.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	grpcAddr := envOrDefault("GRPC_ADDR", defaultGRPCAddr)

	listener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return err
	}
	defer listener.Close()

	grpcServer := grpc.NewServer()
	createPaymentUC := usecase.NewCreatePaymentUsecase(nil, nil, nil, nil, nil)
	payment.RegisterPaymentServiceServer(grpcServer, grpc.NewPaymentServer(createPaymentUC))

	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("grpc server listening", "addr", grpcAddr)
		serveErr <- grpcServer.Serve(listener)
	}()

	signalCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serveErr:
		return err
	case <-signalCtx.Done():
		healthServer.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
		grpc.GracefulShutdown(grpcServer, grpcShutdownGracePeriod)
		return nil
	}
}

func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}
