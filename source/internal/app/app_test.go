package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"example.edu/6cc545/cw1/internal/config"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func useTemporaryWorkingDirectory(t *testing.T) {
	t.Helper()

	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
}

func TestWeek01_OccupiedPort(t *testing.T) {
	useTemporaryWorkingDirectory(t)
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer ln.Close()
	c := config.Config{ID: "node-a", HTTP: ln.Addr().String(), Timeout: time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	e = Run(ctx, c)
	if e == nil || !strings.Contains(e.Error(), "listen HTTP") {
		t.Fatalf("want clear bind error, got %v", e)
	}
}
func TestWeek01_CancelledLifecycle(t *testing.T) {
	useTemporaryWorkingDirectory(t)
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	address := ln.Addr().String()
	_ = ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := config.Config{ID: "node-a", HTTP: address, Timeout: time.Second}
	if e = Run(ctx, c); e != nil {
		t.Fatal(e)
	}
}

func TestWireStoresFilesThroughHTTP(t *testing.T) {
	useTemporaryWorkingDirectory(t)

	mux := http.NewServeMux()
	ctx, cancel := context.WithCancel(context.Background())
	stop, err := Wire(ctx, config.Config{
		ID:      "node-test",
		Timeout: time.Second,
	}, mux)
	if err != nil {
		cancel()
		t.Fatalf("Wire: %v", err)
	}

	server := httptest.NewServer(mux)
	t.Cleanup(func() {
		server.Close()
		stop()
		cancel()
	})

	name := "integration.txt"
	content := []byte("Real application storage test")
	sum := sha256.Sum256(content)
	digest := hex.EncodeToString(sum[:])

	req, err := http.NewRequest(
		http.MethodPost,
		server.URL+"/v1/files/"+name,
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-File-Size", fmt.Sprint(len(content)))
	req.Header.Set("X-File-SHA256", digest)

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload request: %v", err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Fatalf("upload status = %d, want %d", response.StatusCode, http.StatusCreated)
	}

	listResponse, err := http.Get(server.URL + "/v1/files")
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	listBody, readErr := io.ReadAll(listResponse.Body)
	_ = listResponse.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if listResponse.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.StatusCode, listBody)
	}
	if !bytes.Contains(listBody, []byte(name)) {
		t.Fatalf("file listing does not contain %q: %s", name, listBody)
	}

	downloadResponse, err := http.Get(server.URL + "/v1/files/" + name)
	if err != nil {
		t.Fatalf("download request: %v", err)
	}
	downloaded, readErr := io.ReadAll(downloadResponse.Body)
	_ = downloadResponse.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}

	if downloadResponse.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d", downloadResponse.StatusCode)
	}
	if !bytes.Equal(downloaded, content) {
		t.Fatalf("downloaded content = %q, want %q", downloaded, content)
	}
	if got := downloadResponse.Header.Get("X-File-SHA256"); got != digest {
		t.Fatalf("download SHA256 = %q, want %q", got, digest)
	}
}
