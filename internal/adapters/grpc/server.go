package grpc

import (
	"time"

	grpcpkg "google.golang.org/grpc"
)

func NewServer(opts ...grpcpkg.ServerOption) *grpcpkg.Server {
	serverOptions := []grpcpkg.ServerOption{
		grpcpkg.ChainUnaryInterceptor(
			recoveryInterceptor,
			loggingInterceptor,
		),
	}
	serverOptions = append(serverOptions, opts...)

	return grpcpkg.NewServer(serverOptions...)
}

func GracefulShutdown(srv *grpcpkg.Server, timeout time.Duration) {
	if timeout <= 0 {
		srv.Stop()
		return
	}

	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(timeout):
		srv.Stop()
	}
}
