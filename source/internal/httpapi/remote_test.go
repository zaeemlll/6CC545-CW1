package httpapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pb "example.edu/6cc545/cw1/api/files/v1"
	"example.edu/6cc545/cw1/internal/config"
	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/rpcfiles"
	"example.edu/6cc545/cw1/internal/security"
	"example.edu/6cc545/cw1/internal/state"
	jwt "github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
)

func TestWeek05_RemoteFilesValidation(t *testing.T) {
	h := RemoteFiles(state.NewRegistry(), "node-a", time.Second, config.Config{})

	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"wrong method", http.MethodPost, "/v1/remote-files?peer_id=node-b", http.StatusMethodNotAllowed},
		{"missing peer", http.MethodGet, "/v1/remote-files", http.StatusBadRequest},
		{"invalid peer", http.MethodGet, "/v1/remote-files?peer_id=bad%20id", http.StatusBadRequest},
		{"unknown peer", http.MethodGet, "/v1/remote-files?peer_id=node-b", http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d; body: %s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.name == "wrong method" && rec.Header().Get("Allow") != "GET" {
				t.Fatalf("Allow header = %q, want GET", rec.Header().Get("Allow"))
			}
		})
	}
}
func TestWeek08_RemoteFilesSuccessfulGRPCRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	catalogue := state.NewCatalog()
	if err := catalogue.Replace([]model.FileMetadata{
		{
			Name:         "remote.txt",
			Size:         5,
			SHA256:       strings.Repeat("a", 64),
			ModifiedUnix: 123,
		},
	}); err != nil {
		t.Fatal(err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterFileServiceServer(grpcServer, &rpcfiles.Service{
		NodeID:  "node-b",
		Catalog: catalogue,
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = grpcServer.Serve(listener)
	}()

	defer func() {
		grpcServer.Stop()
		_ = listener.Close()
		<-done
	}()

	registry := state.NewRegistry()
	peer := model.Peer{
		ID:          "node-b",
		HTTPAddress: "127.0.0.1:8082",
		GRPCAddress: listener.Addr().String(),
		Status:      "unknown",
	}
	if err := registry.Add(peer); err != nil {
		t.Fatal(err)
	}

	handler := RemoteFiles(registry, "node-a", time.Second, config.Config{})
	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/remote-files?peer_id=node-b",
		nil,
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s",
			response.Code, http.StatusOK, response.Body.String())
	}

	var result struct {
		NodeID string               `json:"node_id"`
		Files  []model.FileMetadata `json:"files"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}

	if result.NodeID != "node-b" {
		t.Fatalf("node ID = %q, want node-b", result.NodeID)
	}
	if len(result.Files) != 1 || result.Files[0].Name != "remote.txt" {
		t.Fatalf("unexpected remote files: %+v", result.Files)
	}
}

type slowRemoteFileService struct {
	pb.UnimplementedFileServiceServer
}

func (slowRemoteFileService) ListFiles(
	_ context.Context,
	_ *pb.ListFilesRequest,
) (*pb.ListFilesResponse, error) {
	time.Sleep(100 * time.Millisecond)
	return &pb.ListFilesResponse{NodeId: "node-b"}, nil
}

func TestWeek08_RemoteFilesUnavailablePeer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}

	registry := state.NewRegistry()
	if err := registry.Add(model.Peer{
		ID:          "node-b",
		HTTPAddress: "127.0.0.1:8082",
		GRPCAddress: address,
		Status:      "unknown",
	}); err != nil {
		t.Fatal(err)
	}

	handler := RemoteFiles(registry, "node-a", time.Second, config.Config{})
	request := httptest.NewRequest(
		http.MethodGet, "/v1/remote-files?peer_id=node-b", nil,
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body: %s",
			response.Code, http.StatusServiceUnavailable, response.Body.String())
	}
}

func TestWeek08_RemoteFilesTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterFileServiceServer(grpcServer, slowRemoteFileService{})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = grpcServer.Serve(listener)
	}()

	defer func() {
		grpcServer.Stop()
		_ = listener.Close()
		<-done
	}()

	registry := state.NewRegistry()
	if err := registry.Add(model.Peer{
		ID:          "node-b",
		HTTPAddress: "127.0.0.1:8082",
		GRPCAddress: listener.Addr().String(),
		Status:      "unknown",
	}); err != nil {
		t.Fatal(err)
	}

	handler := RemoteFiles(registry, "node-a", 10*time.Millisecond, config.Config{})
	request := httptest.NewRequest(
		http.MethodGet, "/v1/remote-files?peer_id=node-b", nil,
	)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d; body: %s",
			response.Code, http.StatusGatewayTimeout, response.Body.String())
	}
}

func TestWeek09_RemoteFilesForwardsJWTToGRPC(t *testing.T) {
	key := []byte("local-test-only-32-byte-randomness-not-for-deployment")
	now := time.Unix(1800000000, 0)

	claims := security.Claims{
		Role: "reader",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "student",
			Issuer:    "cw2-lab",
			Audience:  jwt.ClaimStrings{"cw2-api"},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			NotBefore: jwt.NewNumericDate(now.Add(-time.Minute)),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	validator := security.TokenValidator{
		Key:      key,
		Issuer:   "cw2-lab",
		Audience: "cw2-api",
		Now:      func() time.Time { return now },
	}

	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(security.GRPCUnary(validator, "reader")),
	)
	pb.RegisterFileServiceServer(grpcServer, &rpcfiles.Service{
		NodeID:  "node-b",
		Catalog: state.NewCatalog(),
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = grpcServer.Serve(listener)
	}()

	defer func() {
		grpcServer.Stop()
		_ = listener.Close()
		<-done
	}()

	registry := state.NewRegistry()
	if err := registry.Add(model.Peer{
		ID:          "node-b",
		HTTPAddress: "127.0.0.1:8082",
		GRPCAddress: listener.Addr().String(),
		Status:      "unknown",
	}); err != nil {
		t.Fatal(err)
	}

	handler := RemoteFiles(registry, "node-a", time.Second, config.Config{})
	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/remote-files?peer_id=node-b",
		nil,
	)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s",
			response.Code, http.StatusOK, response.Body.String())
	}
}
