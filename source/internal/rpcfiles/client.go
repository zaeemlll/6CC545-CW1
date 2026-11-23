package rpcfiles

import (
	"context"
	"time"

	pb "example.edu/6cc545/cw1/api/files/v1"
	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func Fetch(
	ctx context.Context,
	address string,
	peerID string,
	requester string,
	timeout time.Duration,
	c config.Config,
) (*pb.ListFilesResponse, error) {
	return FetchWithToken(ctx, address, peerID, requester, timeout, "", c)
}

func FetchWithToken(
	ctx context.Context,
	address string,
	peerID string,
	requester string,
	timeout time.Duration,
	token string,
	c config.Config,
) (*pb.ListFilesResponse, error) {
	if !config.ValidPeerAddress(address) {
		return nil, status.Error(codes.InvalidArgument, "invalid peer address")
	}

	if !config.ValidID(peerID) || !config.ValidID(requester) {
		return nil, status.Error(codes.InvalidArgument, "invalid peer or requester ID")
	}

	if timeout <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid RPC timeout")
	}

	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	options := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(4096),
			grpc.MaxCallRecvMsgSize(65536),
		),
	}

	if c.SecurityMode == "required" {
		tlsConfig, err := security.ClientTLS(
			c.TLSCertFile,
			c.TLSKeyFile,
			c.CAFile,
			peerID,
		)
		if err != nil {
			return nil, status.Error(codes.FailedPrecondition, "gRPC TLS configuration failed")
		}

		options = append(options, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		options = append(options, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(address, options...)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewFileServiceClient(conn)

	callCtx := opCtx
	if token != "" {
		callCtx = metadata.AppendToOutgoingContext(
			callCtx,
			"authorization",
			"Bearer "+token,
		)
	}

	return client.ListFiles(callCtx, &pb.ListFilesRequest{
		RequesterId: requester,
	})
}
