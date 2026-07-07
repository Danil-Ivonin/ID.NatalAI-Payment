package grpc

import (
	"context"
	"log/slog"
	"time"

	grpcpkg "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func loggingInterceptor(
	ctx context.Context,
	req any,
	info *grpcpkg.UnaryServerInfo,
	handler grpcpkg.UnaryHandler,
) (any, error) {
	start := time.Now()

	resp, err := handler(ctx, req)

	slog.InfoContext(
		ctx,
		"grpc request",
		"method", info.FullMethod,
		"duration", time.Since(start),
		"code", status.Code(err).String(),
	)

	return resp, err
}

func recoveryInterceptor(
	ctx context.Context,
	req any,
	info *grpcpkg.UnaryServerInfo,
	handler grpcpkg.UnaryHandler,
) (resp any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(ctx, "grpc panic recovered", "method", info.FullMethod, "panic", recovered)
			err = status.Error(codes.Internal, "internal server error")
		}
	}()

	return handler(ctx, req)
}
