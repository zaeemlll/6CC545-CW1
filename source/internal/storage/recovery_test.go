package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func recoveryDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestRecoverRemovesAbandonedTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}

	body := []byte("committed")
	if _, err := store.Put(context.Background(), "keep.txt",
		bytes.NewReader(body), int64(len(body)), recoveryDigest(body)); err != nil {
		t.Fatal(err)
	}

	tempPath := filepath.Join(dir, ".w07-tmp-abandoned")
	if err := os.WriteFile(tempPath, []byte("unfinished"), 0600); err != nil {
		t.Fatal(err)
	}

	restarted, err := New(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary file was not removed: %v", err)
	}

	reader, metadata, err := restarted.Open("keep.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	var got bytes.Buffer
	if _, err := got.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), body) || metadata.SHA256 != recoveryDigest(body) {
		t.Fatal("committed file was not recovered correctly")
	}
}

func TestRecoverRejectsSymbolicLinks(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linked.txt")); err != nil {
		t.Fatal(err)
	}

	store, err := New(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err == nil {
		t.Fatal("recovery accepted a symbolic link")
	}

	files, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("failed recovery published partial metadata: %+v", files)
	}
}

func TestRecoverHonoursCancelledContext(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := store.Recover(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}

	files, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("cancelled recovery changed the catalogue: %+v", files)
	}
}
