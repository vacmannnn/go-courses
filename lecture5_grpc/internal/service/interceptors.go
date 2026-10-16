package service

import (
	"context"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func requestID(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	values := md.Get("x-request-id")
	if len(values) == 0 {
		return "-"
	}
	return values[0]
}

func UnaryLogging(logger *log.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		logger.Printf("method=%s request_id=%q code=%s duration=%s",
			info.FullMethod, requestID(ctx), status.Code(err), time.Since(start))
		return resp, err
	}
}

// A unary interceptor does not intercept streaming RPCs.
func StreamLogging(logger *log.Logger) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, stream)
		logger.Printf("method=%s request_id=%q code=%s duration=%s",
			info.FullMethod, requestID(stream.Context()), status.Code(err), time.Since(start))
		return err
	}
}
