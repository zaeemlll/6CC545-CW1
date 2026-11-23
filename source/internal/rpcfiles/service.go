package rpcfiles

import (
	"context"

	pb "example.edu/6cc545/cw1/api/files/v1"
	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type Catalog interface {
	Snapshot() []model.FileMetadata
}

type Service struct {
	pb.UnimplementedFileServiceServer
	NodeID              string
	Catalog             Catalog
	RequirePeerIdentity bool
}

func (s *Service) ListFiles(ctx context.Context, r *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}

	if r == nil || !config.ValidID(r.GetRequesterId()) {
		return nil, status.Error(codes.InvalidArgument, "invalid requester_id")
	}

	if s.RequirePeerIdentity {
		remote, ok := peer.FromContext(ctx)
		if !ok || remote.AuthInfo == nil {
			return nil, status.Error(codes.Unauthenticated, "verified peer identity required")
		}

		tlsInfo, ok := remote.AuthInfo.(credentials.TLSInfo)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "verified TLS peer identity required")
		}

		if len(tlsInfo.State.VerifiedChains) == 0 ||
			len(tlsInfo.State.VerifiedChains[0]) == 0 {
			return nil, status.Error(codes.Unauthenticated, "verified client certificate required")
		}

		leaf := tlsInfo.State.VerifiedChains[0][0]
		expectedURI := "spiffe://cw2.local/" + r.GetRequesterId()
		if len(leaf.URIs) != 1 || leaf.URIs[0].String() != expectedURI {
			return nil, status.Error(codes.PermissionDenied, "requester identity does not match client certificate")
		}
	}

	if s.Catalog == nil {
		return nil, status.Error(codes.Internal, "catalogue unavailable")
	}

	snapshot := s.Catalog.Snapshot()
	files := make([]*pb.FileMetadata, 0, len(snapshot))

	for _, f := range snapshot {
		files = append(files, &pb.FileMetadata{
			Name:         f.Name,
			Size:         f.Size,
			Sha256:       f.SHA256,
			ModifiedUnix: f.ModifiedUnix,
		})
	}

	return &pb.ListFilesResponse{
		NodeId: s.NodeID,
		Files:  files,
	}, nil
}
