package rpcfiles

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"example.edu/6cc545/cw1/internal/storage"
)

func TestNewStorageCatalogRejectsNilStore(t *testing.T) {
	_, err := NewStorageCatalog(nil)
	if err == nil {
		t.Fatal("expected an error for a nil store")
	}
}

func TestStorageCatalogReturnsCommittedMetadata(t *testing.T) {
	store, err := storage.New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}

	catalog, err := NewStorageCatalog(store)
	if err != nil {
		t.Fatal(err)
	}

	if got := catalog.Snapshot(); len(got) != 0 {
		t.Fatalf("expected an empty catalogue before a write, got %+v", got)
	}

	content := []byte("Week 7 committed file")
	sum := sha256.Sum256(content)
	expectedSHA := hex.EncodeToString(sum[:])

	committed, err := store.Put(
		context.Background(),
		"week07.txt",
		bytes.NewReader(content),
		int64(len(content)),
		expectedSHA,
	)
	if err != nil {
		t.Fatalf("store Put: %v", err)
	}

	first := catalog.Snapshot()
	if len(first) != 1 {
		t.Fatalf("expected one stored file, got %d", len(first))
	}

	got := first[0]
	if got.Name != committed.Name ||
		got.Size != committed.Size ||
		got.SHA256 != committed.SHA256 ||
		got.ModifiedUnix != committed.ModifiedUnix {
		t.Fatalf("adapter metadata = %+v, committed metadata = %+v", got, committed)
	}

	second := catalog.Snapshot()
	if len(second) != 1 {
		t.Fatalf("expected one file in second snapshot, got %d", len(second))
	}
	if second[0].ModifiedUnix != first[0].ModifiedUnix {
		t.Fatalf("timestamp changed between snapshots: %d and %d",
			first[0].ModifiedUnix, second[0].ModifiedUnix)
	}
}
