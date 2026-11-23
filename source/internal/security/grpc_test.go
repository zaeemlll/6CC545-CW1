//go:build acceptance

package security

import (
	"context"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestGRPCUnaryJWTAuthentication(t *testing.T) {
	v := validator()
	validReader := signed(t, claims("reader"), testKey, jwt.SigningMethodHS256)
	validWriter := signed(t, claims("writer"), testKey, jwt.SigningMethodHS256)

	expiredClaims := claims("reader")
	expiredClaims.ExpiresAt = jwt.NewNumericDate(testTime.Add(-time.Second))
	expiredToken := signed(t, expiredClaims, testKey, jwt.SigningMethodHS256)

	tests := []struct {
		name         string
		token        string
		requiredRole string
		want         codes.Code
	}{
		{"missing token", "", "reader", codes.Unauthenticated},
		{"invalid token", "not-a-jwt", "reader", codes.Unauthenticated},
		{"expired token", expiredToken, "reader", codes.Unauthenticated},
		{"valid reader", validReader, "reader", codes.OK},
		{"valid writer for read", validWriter, "reader", codes.OK},
		{"reader cannot write", validReader, "writer", codes.PermissionDenied},
		{"writer can write", validWriter, "writer", codes.OK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.token != "" {
				ctx = metadata.NewIncomingContext(
					ctx,
					metadata.Pairs("authorization", "Bearer "+tc.token),
				)
			}

			called := false
			interceptor := GRPCUnary(v, tc.requiredRole)

			_, err := interceptor(
				ctx,
				nil,
				&grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"},
				func(context.Context, interface{}) (interface{}, error) {
					called = true
					return "success", nil
				},
			)

			if status.Code(err) != tc.want {
				t.Fatalf("got %v, want %v; error: %v", status.Code(err), tc.want, err)
			}

			if called != (tc.want == codes.OK) {
				t.Fatalf("handler called=%v, want %v", called, tc.want == codes.OK)
			}
		})
	}
}
