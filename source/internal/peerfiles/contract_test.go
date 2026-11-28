//go:build acceptance

package peerfiles

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"example.edu/6cc545/cw1/internal/security"
	"example.edu/6cc545/cw1/internal/storage"
)

func TestWeek10_TruncatedHTTPSDownloadPreservesExistingFile(t *testing.T) {
	cfg, serverCert, serverKey, _, caPath := peerTransferTLSConfig(t)

	destination, err := storage.New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := destination.EnableVersioning("node-c"); err != nil {
		t.Fatal(err)
	}
	if err := destination.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	oldBody := []byte("previous valid content")
	oldDigest := sha256.Sum256(oldBody)
	oldMetadata, err := destination.PutLocalVersioned(
		context.Background(),
		"report.txt",
		bytes.NewReader(oldBody),
		int64(len(oldBody)),
		hex.EncodeToString(oldDigest[:]),
	)
	if err != nil {
		t.Fatal(err)
	}

	newBody := []byte("partial")
	newDigest := sha256.Sum256([]byte("complete replacement content"))

	serverTLS, err := security.ServerTLS(serverCert, serverKey, caPath, true)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-File-Size", strconv.Itoa(len("complete replacement content")))
		w.Header().Set("X-File-SHA256", hex.EncodeToString(newDigest[:]))
		w.Header().Set("X-File-Version", "2")
		w.Header().Set("X-File-Writer-ID", "node-b")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(newBody)
	}))
	server.TLS = serverTLS
	server.StartTLS()
	defer server.Close()

	address := server.Listener.Addr().(*net.TCPAddr)
	peerURL := "https://localhost:" + strconv.Itoa(address.Port)

	expected := storage.Metadata{
		Name:     "report.txt",
		Size:     int64(len("complete replacement content")),
		SHA256:   hex.EncodeToString(newDigest[:]),
		Version:  2,
		WriterID: "node-b",
	}

	_, err = DownloadExpected(
		context.Background(),
		peerURL,
		"localhost",
		"report.txt",
		destination,
		3*time.Second,
		cfg,
		expected,
	)
	if err == nil {
		t.Fatal("truncated HTTPS download succeeded")
	}

	reader, actualMetadata, err := destination.Open("report.txt")
	if err != nil {
		t.Fatalf("open existing file after failed download: %v", err)
	}
	defer reader.Close()

	actualBody, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(actualBody) != string(oldBody) {
		t.Fatalf("existing content changed: got %q, want %q", actualBody, oldBody)
	}
	if actualMetadata != oldMetadata {
		t.Fatalf("existing metadata changed: got %+v, want %+v", actualMetadata, oldMetadata)
	}

}

func TestWeek10_CancelledHTTPSDownloadPreservesExistingFile(t *testing.T) {
	cfg, serverCert, serverKey, _, caPath := peerTransferTLSConfig(t)

	destination, err := storage.New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := destination.EnableVersioning("node-c"); err != nil {
		t.Fatal(err)
	}
	if err := destination.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	oldBody := []byte("previous valid content")
	oldDigest := sha256.Sum256(oldBody)
	oldMetadata, err := destination.PutLocalVersioned(
		context.Background(),
		"report.txt",
		bytes.NewReader(oldBody),
		int64(len(oldBody)),
		hex.EncodeToString(oldDigest[:]),
	)
	if err != nil {
		t.Fatal(err)
	}

	fullBody := []byte("complete replacement content")
	newDigest := sha256.Sum256(fullBody)
	responseStarted := make(chan struct{})

	serverTLS, err := security.ServerTLS(serverCert, serverKey, caPath, true)
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", strconv.Itoa(len(fullBody)))
		w.Header().Set("X-File-Size", strconv.Itoa(len(fullBody)))
		w.Header().Set("X-File-SHA256", hex.EncodeToString(newDigest[:]))
		w.Header().Set("X-File-Version", "2")
		w.Header().Set("X-File-Writer-ID", "node-b")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		close(responseStarted)

		<-r.Context().Done()
	}))
	server.TLS = serverTLS
	server.StartTLS()
	defer server.Close()

	address := server.Listener.Addr().(*net.TCPAddr)
	peerURL := "https://localhost:" + strconv.Itoa(address.Port)

	expected := storage.Metadata{
		Name:     "report.txt",
		Size:     int64(len(fullBody)),
		SHA256:   hex.EncodeToString(newDigest[:]),
		Version:  2,
		WriterID: "node-b",
	}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)

	go func() {
		_, err := DownloadExpected(
			ctx,
			peerURL,
			"localhost",
			"report.txt",
			destination,
			5*time.Second,
			cfg,
			expected,
		)
		result <- err
	}()

	select {
	case <-responseStarted:
		cancel()
	case <-time.After(5 * time.Second):
		cancel()
		t.Fatal("server did not start sending the response")
	}

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("cancelled HTTPS download succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("download did not stop after cancellation")
	}

	reader, actualMetadata, err := destination.Open("report.txt")
	if err != nil {
		t.Fatalf("open existing file after cancelled download: %v", err)
	}
	defer reader.Close()

	actualBody, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(actualBody) != string(oldBody) {
		t.Fatalf("existing content changed: got %q, want %q", actualBody, oldBody)
	}
	if actualMetadata != oldMetadata {
		t.Fatalf("existing metadata changed: got %+v, want %+v", actualMetadata, oldMetadata)
	}
}

func TestWeek10_OversizedHTTPSDownloadPreservesExistingFile(t *testing.T) {
	cfg, serverCert, serverKey, _, caPath := peerTransferTLSConfig(t)

	destination, err := storage.New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := destination.EnableVersioning("node-c"); err != nil {
		t.Fatal(err)
	}
	if err := destination.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	oldBody := []byte("previous valid content")
	oldDigest := sha256.Sum256(oldBody)
	oldMetadata, err := destination.PutLocalVersioned(
		context.Background(),
		"report.txt",
		bytes.NewReader(oldBody),
		int64(len(oldBody)),
		hex.EncodeToString(oldDigest[:]),
	)
	if err != nil {
		t.Fatal(err)
	}

	advertisedBody := []byte("replacement")
	actualBody := []byte("replacement-extra")
	newDigest := sha256.Sum256(advertisedBody)

	serverTLS, err := security.ServerTLS(serverCert, serverKey, caPath, true)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-File-Size", strconv.Itoa(len(advertisedBody)))
		w.Header().Set("X-File-SHA256", hex.EncodeToString(newDigest[:]))
		w.Header().Set("X-File-Version", "2")
		w.Header().Set("X-File-Writer-ID", "node-b")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(actualBody)
	}))
	server.EnableHTTP2 = false
	server.TLS = serverTLS
	server.StartTLS()
	defer server.Close()

	address := server.Listener.Addr().(*net.TCPAddr)
	peerURL := "https://localhost:" + strconv.Itoa(address.Port)

	expected := storage.Metadata{
		Name:     "report.txt",
		Size:     int64(len(advertisedBody)),
		SHA256:   hex.EncodeToString(newDigest[:]),
		Version:  2,
		WriterID: "node-b",
	}

	_, err = DownloadExpected(
		context.Background(),
		peerURL,
		"localhost",
		"report.txt",
		destination,
		3*time.Second,
		cfg,
		expected,
	)
	if err == nil {
		t.Fatal("oversized HTTPS download succeeded")
	}

	reader, actualMetadata, err := destination.Open("report.txt")
	if err != nil {
		t.Fatalf("open existing file after oversized download: %v", err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, oldBody) {
		t.Fatalf("existing content changed: got %q, want %q", got, oldBody)
	}
	if actualMetadata != oldMetadata {
		t.Fatalf("existing metadata changed: got %+v, want %+v", actualMetadata, oldMetadata)
	}
}
