package security

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func GRPCUnary(v TokenValidator, requiredRole string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "bearer token required")
		}

		values := md.Get("authorization")
		if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "bearer token required")
		}

		raw := strings.TrimPrefix(values[0], "Bearer ")
		if raw == "" || strings.ContainsAny(raw, " \t\r\n") {
			return nil, status.Error(codes.Unauthenticated, "invalid bearer token")
		}

		if _, err := v.Validate(raw, requiredRole); err != nil {
			if errors.Is(err, ErrForbidden) {
				return nil, status.Error(codes.PermissionDenied, "insufficient permissions")
			}
			return nil, status.Error(codes.Unauthenticated, "invalid bearer token")
		}

		return handler(ctx, req)
	}
}
