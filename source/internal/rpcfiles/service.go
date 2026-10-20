package rpcfiles

import (
	"context"

	pb "example.edu/6cc545/cw1/api/files/v1"
	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Catalog interface {
	Snapshot() []model.FileMetadata
}

type Service struct {
	pb.UnimplementedFileServiceServer
	NodeID  string
	Catalog Catalog
}

func (s *Service) ListFiles(ctx context.Context, r *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}

	if r == nil || !config.ValidID(r.GetRequesterId()) {
		return nil, status.Error(codes.InvalidArgument, "invalid requester_id")
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
