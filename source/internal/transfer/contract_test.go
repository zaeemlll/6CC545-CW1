//go:build acceptance

package transfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"example.edu/6cc545/cw1/internal/storage"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestVerifiedFetchAndTamper(t *testing.T) {
	body := []byte("transfer-test\n")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	client := server.Client()
	client.Timeout = 2 * time.Second
	s, _ := storage.New(t.TempDir(), 1024)
	want := fmt.Sprintf("%x", sha256.Sum256(body))
	req := Request{URL: server.URL + "/files/a.txt", AllowedHost: u.Host, Name: "a.txt", Size: int64(len(body)), SHA256: want}
	if _, e := Fetch(context.Background(), client, s, req); e != nil {
		t.Fatal(e)
	}
	r, m, e := s.Open("a.txt")
	if e != nil {
		t.Fatal(e)
	}
	got, _ := io.ReadAll(r)
	r.Close()
	if !bytes.Equal(got, body) || m.SHA256 != want {
		t.Fatal("bad content")
	}
	req.SHA256 = fmt.Sprintf("%064d", 0)
	if _, e = Fetch(context.Background(), client, s, req); e == nil {
		t.Fatal("tampered expected digest accepted")
	}
}
func TestRejectWrongDestinationAndRedirect(t *testing.T) {
	s, _ := storage.New(t.TempDir(), 1024)
	var plaintextHits, firstHits, redirectHits atomic.Int32
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { plaintextHits.Add(1); fmt.Fprint(w, "x") }))
	defer plain.Close()
	pu, _ := url.Parse(plain.URL)
	r := Request{URL: plain.URL + "/x", AllowedHost: pu.Host, Name: "x.txt", Size: 1, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("x")))}
	if _, e := Fetch(context.Background(), plain.Client(), s, r); e == nil {
		t.Fatal("plaintext accepted")
	}
	if plaintextHits.Load() != 0 {
		t.Fatal("plaintext destination contacted before rejection")
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/target" {
			redirectHits.Add(1)
			fmt.Fprint(w, "x")
			return
		}
		firstHits.Add(1)
		http.Redirect(w, r, "/target", 302)
	}))
	defer server.Close()
	u, _ := url.Parse(server.URL)
	r.URL = server.URL
	r.AllowedHost = u.Host
	if _, e := Fetch(context.Background(), server.Client(), s, r); e == nil {
		t.Fatal("redirect accepted")
	}
	if firstHits.Load() != 1 || redirectHits.Load() != 0 {
		t.Fatalf("redirect request first=%d followed=%d", firstHits.Load(), redirectHits.Load())
	}
	r.AllowedHost = "other-node:9444"
	if _, e := Fetch(context.Background(), server.Client(), s, r); e == nil {
		t.Fatal("host mismatch accepted")
	}
	if firstHits.Load() != 1 {
		t.Fatal("unauthorised host contacted before rejection")
	}
}

func TestVersionedFetchPreservesMetadata(t *testing.T) {
	body := []byte("versioned transfer test")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	store, err := storage.New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-b"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	digest := fmt.Sprintf("%x", sha256.Sum256(body))
	request := Request{
		URL:         server.URL + "/files/report.txt",
		AllowedHost: u.Host,
		Name:        "report.txt",
		Size:        int64(len(body)),
		SHA256:      digest,
		Version:     7,
		WriterID:    "node-a",
	}

	metadata, err := Fetch(context.Background(), server.Client(), store, request)
	if err != nil {
		t.Fatalf("versioned fetch failed: %v", err)
	}
	if metadata.Version != 7 || metadata.WriterID != "node-a" {
		t.Fatalf("metadata = %+v, want version 7 and writer node-a", metadata)
	}

	reader, stored, err := store.Open("report.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("stored content = %q, want %q", got, body)
	}
	if stored.Version != 7 || stored.WriterID != "node-a" {
		t.Fatalf("stored metadata = %+v, want version 7 and writer node-a", stored)
	}

	request.Version = 0
	if _, err := Fetch(context.Background(), server.Client(), store, request); err == nil {
		t.Fatal("accepted missing version metadata")
	}
}
