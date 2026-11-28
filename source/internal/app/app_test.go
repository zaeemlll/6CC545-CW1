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
	"path/filepath"
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

func TestWeek08_WireRecoversFilesThroughHTTP(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "node-data")
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		t.Fatal(err)
	}

	name := "restored.txt"
	content := []byte("File restored after node restart")
	if err := os.WriteFile(filepath.Join(dataDir, name), content, 0600); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	ctx, cancel := context.WithCancel(context.Background())
	stop, err := Wire(ctx, config.Config{
		ID:      "node-recovery",
		Timeout: time.Second,
		DataDir: dataDir,
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

	response, err := http.Get(server.URL + "/v1/files/" + name)
	if err != nil {
		t.Fatalf("download recovered file: %v", err)
	}
	defer response.Body.Close()

	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d, body = %s", response.StatusCode, got)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("recovered content = %q, want %q", got, content)
	}
}

func TestWeek08_WireRegistersPeerSeeds(t *testing.T) {
	mux := http.NewServeMux()
	ctx, cancel := context.WithCancel(context.Background())
	stop, err := Wire(ctx, config.Config{
		ID:        "node-a",
		Timeout:   time.Second,
		DataDir:   filepath.Join(t.TempDir(), "node-a"),
		PeerSeeds: []string{"node-b:9090", "node-c:9090"},
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

	response, err := http.Get(server.URL + "/v1/peers")
	if err != nil {
		t.Fatalf("get peers: %v", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("peer status = %d, body = %s", response.StatusCode, body)
	}

	for _, expected := range []string{
		"node-b", "node-b:8080", "node-b:9090",
		"node-c", "node-c:8080", "node-c:9090",
	} {
		if !bytes.Contains(body, []byte(expected)) {
			t.Errorf("peer response does not contain %q: %s", expected, body)
		}
	}
}

func TestWeek08_WireRejectsInvalidPeerSeeds(t *testing.T) {
	tests := []struct {
		name  string
		seeds []string
	}{
		{
			name:  "invalid seed",
			seeds: []string{"not a valid address"},
		},
		{
			name:  "self reference",
			seeds: []string{"node-a:9090"},
		},
		{
			name:  "duplicate seed",
			seeds: []string{"node-b:9090", "node-b:9090"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, err := Wire(ctx, config.Config{
				ID:        "node-a",
				Timeout:   time.Second,
				DataDir:   filepath.Join(t.TempDir(), "node-a"),
				PeerSeeds: test.seeds,
			}, http.NewServeMux())

			if err == nil {
				t.Fatal("Wire succeeded; want peer seed validation error")
			}
		})
	}
}

func TestWeek09_PeerFromSeed(t *testing.T) {
	tests := []struct {
		name     string
		seed     string
		wantID   string
		wantHTTP string
		wantGRPC string
		wantErr  bool
	}{
		{
			name:     "Week 8 host and port",
			seed:     "node-b:9092",
			wantID:   "node-b",
			wantHTTP: "node-b:8080",
			wantGRPC: "node-b:9092",
		},
		{
			name:     "Week 9 HTTPS seed",
			seed:     "https://node-b:9444",
			wantID:   "node-b",
			wantHTTP: "https://node-b:9444",
			wantGRPC: "node-b:9443",
		},
		{
			name:    "insecure HTTP seed",
			seed:    "http://node-b:9444",
			wantErr: true,
		},
		{
			name:    "HTTPS seed missing port",
			seed:    "https://node-b",
			wantErr: true,
		},
		{
			name:    "HTTPS seed with path",
			seed:    "https://node-b:9444/files",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			peer, err := peerFromSeed(tc.seed)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected invalid seed to be rejected")
				}
				return
			}

			if err != nil {
				t.Fatalf("peerFromSeed returned an error: %v", err)
			}
			if peer.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", peer.ID, tc.wantID)
			}
			if peer.HTTPAddress != tc.wantHTTP {
				t.Errorf("HTTPAddress = %q, want %q", peer.HTTPAddress, tc.wantHTTP)
			}
			if peer.GRPCAddress != tc.wantGRPC {
				t.Errorf("GRPCAddress = %q, want %q", peer.GRPCAddress, tc.wantGRPC)
			}
		})
	}
}

func TestWeek10_RunShutsDownAfterRuntimeCancellation(t *testing.T) {
	useTemporaryWorkingDirectory(t)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, config.Config{
			ID:      "node-test",
			HTTP:    address,
			Timeout: time.Second,
		})
	}()

	client := http.Client{Timeout: 200 * time.Millisecond}
	url := "http://" + address + "/healthz"
	startupDeadline := time.Now().Add(3 * time.Second)

	for {
		response, requestErr := client.Get(url)
		if requestErr == nil {
			_ = response.Body.Close()
			if response.StatusCode != http.StatusOK {
				cancel()
				t.Fatalf("health check returned status %d", response.StatusCode)
			}
			break
		}

		select {
		case runErr := <-done:
			t.Fatalf("application stopped before becoming ready: %v", runErr)
		default:
		}

		if time.Now().After(startupDeadline) {
			cancel()
			t.Fatalf("application did not become ready: %v", requestErr)
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()

	select {
	case runErr := <-done:
		if runErr != nil {
			t.Fatalf("application shutdown returned an error: %v", runErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("application did not shut down after cancellation")
	}
}
