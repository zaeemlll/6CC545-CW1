package rpcfiles

import (
	"context"
	"errors"
	pb "example.edu/6cc545/cw1/api/files/v1"
	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net"
	"strings"
	"testing"
	"time"
)

func TestWeek05_RemoteList(t *testing.T) {
	cat := state.NewCatalog()
	if e := cat.Replace([]model.FileMetadata{{Name: "notes.txt", Size: 1, SHA256: strings.Repeat("a", 64), ModifiedUnix: 1}}); e != nil {
		t.Fatal(e)
	}
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	s := grpc.NewServer()
	pb.RegisterFileServiceServer(s, &Service{NodeID: "node-b", Catalog: cat})
	done := make(chan struct{})
	go func() { defer close(done); _ = s.Serve(ln) }()
	defer func() { s.Stop(); _ = ln.Close(); <-done }()
	got, e := Fetch(context.Background(), ln.Addr().String(), "node-b", "node-a", time.Second, config.Config{})
	if e != nil {
		t.Fatal(e)
	}
	if got.GetNodeId() != "node-b" || len(got.GetFiles()) != 1 || got.Files[0].GetName() != "notes.txt" {
		t.Fatalf("wrong remote result: %v", got)
	}
	_, e = Fetch(context.Background(), ln.Addr().String(), "node-b", "", time.Second, config.Config{})
	if status.Code(e) != codes.InvalidArgument {
		t.Fatalf("empty requester: want InvalidArgument got %v", e)
	}
}
func TestWeek05_Unavailable(t *testing.T) {
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	start := time.Now()
	_, e = Fetch(context.Background(), addr, "node-b", "node-a", 250*time.Millisecond, config.Config{})
	if e == nil {
		t.Fatal("unavailable peer returned success")
	}
	if status.Code(e) != codes.Unavailable && status.Code(e) != codes.DeadlineExceeded {
		t.Fatalf("unexpected status: %v", e)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("unbounded RPC")
	}
}
func TestWeek05_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := (&Service{NodeID: "node-b", Catalog: state.NewCatalog()}).ListFiles(ctx, &pb.ListFilesRequest{RequesterId: "node-a"})
	if status.Code(e) != codes.Canceled {
		t.Fatalf("want Canceled got %v", e)
	}
}

func TestWeek05_EmptyCatalogue(t *testing.T) {
	cat := state.NewCatalog()
	svc := &Service{NodeID: "node-empty", Catalog: cat}

	got, err := svc.ListFiles(
		context.Background(),
		&pb.ListFilesRequest{RequesterId: "node-a"},
	)
	if err != nil {
		t.Fatalf("empty catalogue should succeed: %v", err)
	}
	if got.GetNodeId() != "node-empty" {
		t.Fatalf("wrong node ID: %q", got.GetNodeId())
	}
	if len(got.GetFiles()) != 0 {
		t.Fatalf("expected no files, got %v", got.GetFiles())
	}
}
func TestWeek08_ServiceRejectsNilRequest(t *testing.T) {
	svc := &Service{
		NodeID:  "node-b",
		Catalog: state.NewCatalog(),
	}

	_, err := svc.ListFiles(context.Background(), nil)
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status = %v, want InvalidArgument; error: %v",
			status.Code(err), err)
	}
}

func TestWeek08_ServiceRejectsMissingCatalogue(t *testing.T) {
	svc := &Service{
		NodeID: "node-b",
	}

	_, err := svc.ListFiles(
		context.Background(),
		&pb.ListFilesRequest{RequesterId: "node-a"},
	)
	if status.Code(err) != codes.Internal {
		t.Fatalf("status = %v, want Internal; error: %v",
			status.Code(err), err)
	}
}

func TestWeek09_ServiceRequiresVerifiedPeerIdentity(t *testing.T) {
	svc := &Service{
		NodeID:              "node-b",
		Catalog:             state.NewCatalog(),
		RequirePeerIdentity: true,
	}

	_, err := svc.ListFiles(
		context.Background(),
		&pb.ListFilesRequest{RequesterId: "node-a"},
	)
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("status = %v, want Unauthenticated; error: %v",
			status.Code(err), err)
	}
}

type failingSnapshotCatalog struct{}

func (failingSnapshotCatalog) Snapshot() []model.FileMetadata {
	return nil
}

func (failingSnapshotCatalog) SnapshotWithError() ([]model.FileMetadata, error) {
	return nil, errors.New("simulated storage recovery failure")
}

func TestServiceReturnsUnavailableWhenStorageSnapshotFails(t *testing.T) {
	svc := &Service{
		NodeID:  "node-b",
		Catalog: failingSnapshotCatalog{},
	}

	_, err := svc.ListFiles(
		context.Background(),
		&pb.ListFilesRequest{RequesterId: "node-a"},
	)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("status = %v, want Unavailable; error: %v",
			status.Code(err), err)
	}
}
