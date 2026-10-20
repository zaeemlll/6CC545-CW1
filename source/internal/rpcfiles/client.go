package rpcfiles

import (
	"context"
	"time"

	pb "example.edu/6cc545/cw1/api/files/v1"
	"example.edu/6cc545/cw1/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func Fetch(ctx context.Context, address, requester string, timeout time.Duration) (*pb.ListFilesResponse, error) {
	if !config.ValidAddress(address) {
		return nil, status.Error(codes.InvalidArgument, "invalid peer address")
	}

	if timeout <= 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid RPC timeout")
	}

	opCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(4096),
			grpc.MaxCallRecvMsgSize(65536),
		),
	)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := pb.NewFileServiceClient(conn)

	return client.ListFiles(opCtx, &pb.ListFilesRequest{
		RequesterId: requester,
	})
}
