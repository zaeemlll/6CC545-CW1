package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"example.edu/6cc545/cw1/internal/model"
	"example.edu/6cc545/cw1/internal/state"
	"example.edu/6cc545/cw1/internal/storage"
)

func peerFileTestCertificate(t *testing.T, id string) *x509.Certificate {
	t.Helper()

	identity, err := url.Parse("spiffe://cw2.local/" + id)
	if err != nil {
		t.Fatal(err)
	}

	return &x509.Certificate{
		URIs: []*url.URL{identity},
	}
}

func peerFileTestHandler(t *testing.T) (*storage.Store, *state.Registry, http.HandlerFunc) {
	t.Helper()

	store, err := storage.New(t.TempDir(), 1024)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}

	registry := state.NewRegistry()
	if err := registry.Add(model.Peer{
		ID:          "node-b",
		HTTPAddress: "127.0.0.1:8082",
		GRPCAddress: "127.0.0.1:9092",
		Status:      "unknown",
	}); err != nil {
		t.Fatalf("register peer: %v", err)
	}

	return store, registry, PeerFiles(store, registry, 1024)
}

func TestPeerFilesRejectsMissingVerifiedCertificate(t *testing.T) {
	_, _, handler := peerFileTestHandler(t)

	request := httptest.NewRequest(http.MethodGet, "/v1/peer/files/report.txt", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestPeerFilesRejectsUnregisteredPeer(t *testing.T) {
	_, _, handler := peerFileTestHandler(t)

	request := httptest.NewRequest(http.MethodGet, "/v1/peer/files/report.txt", nil)
	request.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{
			peerFileTestCertificate(t, "node-c"),
		}},
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestPeerFilesRegisteredPeerUploadAndDownload(t *testing.T) {
	_, _, handler := peerFileTestHandler(t)

	content := []byte("private peer transfer")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	upload := httptest.NewRequest(
		http.MethodPost,
		"/v1/peer/files/report.txt",
		bytes.NewReader(content),
	)
	upload.Header.Set("X-File-Size", "21")
	upload.Header.Set("X-File-SHA256", digest)
	upload.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{
			peerFileTestCertificate(t, "node-b"),
		}},
	}

	uploadResponse := httptest.NewRecorder()
	handler.ServeHTTP(uploadResponse, upload)

	if uploadResponse.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body: %s",
			uploadResponse.Code, http.StatusCreated, uploadResponse.Body.String())
	}

	download := httptest.NewRequest(
		http.MethodGet,
		"/v1/peer/files/report.txt",
		nil,
	)
	download.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{
			peerFileTestCertificate(t, "node-b"),
		}},
	}

	downloadResponse := httptest.NewRecorder()
	handler.ServeHTTP(downloadResponse, download)

	if downloadResponse.Code != http.StatusOK {
		t.Fatalf("download status = %d, want %d; body: %s",
			downloadResponse.Code, http.StatusOK, downloadResponse.Body.String())
	}
	if !bytes.Equal(downloadResponse.Body.Bytes(), content) {
		t.Fatalf("downloaded content = %q, want %q",
			downloadResponse.Body.Bytes(), content)
	}
	if downloadResponse.Header().Get("X-File-SHA256") != digest {
		t.Fatal("download response has incorrect SHA-256 header")
	}
}

func TestPeerFilesPreservesVersionedMetadata(t *testing.T) {
	store, _, handler := peerFileTestHandler(t)

	if err := store.EnableVersioning("node-c"); err != nil {
		t.Fatalf("enable versioning: %v", err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatalf("recover versioned store: %v", err)
	}

	content := []byte("remote versioned file")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	upload := httptest.NewRequest(
		http.MethodPost, "/v1/peer/files/report.txt", bytes.NewReader(content),
	)
	upload.Header.Set("X-File-Size", "21")
	upload.Header.Set("X-File-SHA256", digest)
	upload.Header.Set("X-File-Version", "7")
	upload.Header.Set("X-File-Writer-ID", "node-a")
	upload.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{
			peerFileTestCertificate(t, "node-b"),
		}},
	}

	uploadResponse := httptest.NewRecorder()
	handler.ServeHTTP(uploadResponse, upload)

	if uploadResponse.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body: %s",
			uploadResponse.Code, http.StatusCreated, uploadResponse.Body.String())
	}

	download := httptest.NewRequest(
		http.MethodGet, "/v1/peer/files/report.txt", nil,
	)
	download.TLS = &tls.ConnectionState{
		VerifiedChains: [][]*x509.Certificate{{
			peerFileTestCertificate(t, "node-b"),
		}},
	}

	downloadResponse := httptest.NewRecorder()
	handler.ServeHTTP(downloadResponse, download)

	if downloadResponse.Code != http.StatusOK {
		t.Fatalf("download status = %d, want %d; body: %s",
			downloadResponse.Code, http.StatusOK, downloadResponse.Body.String())
	}
	if downloadResponse.Header().Get("X-File-Version") != "7" {
		t.Fatalf("version header = %q, want 7",
			downloadResponse.Header().Get("X-File-Version"))
	}
	if downloadResponse.Header().Get("X-File-Writer-ID") != "node-a" {
		t.Fatalf("writer header = %q, want node-a",
			downloadResponse.Header().Get("X-File-Writer-ID"))
	}
	if !bytes.Equal(downloadResponse.Body.Bytes(), content) {
		t.Fatalf("downloaded content = %q, want %q",
			downloadResponse.Body.Bytes(), content)
	}
}
