//go:build acceptance

package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRecoverCommittedData(t *testing.T) {
	dir := t.TempDir()

	s, err := New(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}

	body := []byte("persist")
	if _, err := s.Put(context.Background(), "keep.txt",
		bytes.NewReader(body), int64(len(body)), digest(body)); err != nil {
		t.Fatal(err)
	}

	s2, err := New(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}

	if err := s2.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	reader, metadata, err := s2.Open("keep.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, body) || metadata.SHA256 != digest(body) {
		t.Fatal("recovery changed committed file")
	}

	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Fatal(err)
	}
}
