package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVersionedRecoveryInitialisesExistingFiles(t *testing.T) {
	dir := t.TempDir()
	body := []byte("existing CW1 file")
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])

	store, err := New(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), "report.txt",
		bytes.NewReader(body), int64(len(body)), digest); err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	files, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Version == 0 || files[0].WriterID != "node-a" {
		t.Fatalf("existing file was not versioned correctly: %+v", files)
	}
}

func TestVersionedRecoveryPreservesManifestAfterRestart(t *testing.T) {
	dir := t.TempDir()
	body := []byte("persistent file")
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])

	first, err := New(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Put(context.Background(), "report.txt",
		bytes.NewReader(body), int64(len(body)), digest); err != nil {
		t.Fatal(err)
	}
	if err := first.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := first.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	before, err := first.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	restarted, err := New(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	after, err := restarted.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 || after[0] != before[0] {
		t.Fatalf("version metadata changed after restart: before=%+v after=%+v", before, after)
	}
}

func TestVersionedRecoveryRejectsMissingManifest(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	manifest := filepath.Join(dir, versionStateDirName, versionManifestName)
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}

	restarted, err := New(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Recover(context.Background()); err == nil {
		t.Fatal("recovery silently recreated a missing version manifest")
	}
}

func TestVersionedStoreBlocksReadsUntilRecovery(t *testing.T) {
	dir := t.TempDir()
	body := []byte("existing file")
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])

	store, err := New(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), "report.txt",
		bytes.NewReader(body), int64(len(body)), digest); err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}

	if _, _, err := store.Open("report.txt"); err == nil {
		t.Fatal("Open exposed a file before versioned recovery")
	}
	if _, err := store.Snapshot(); err == nil {
		t.Fatal("Snapshot exposed the catalogue before versioned recovery")
	}

	if err := store.Recover(context.Background()); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	reader, metadata, err := store.Open("report.txt")
	if err != nil {
		t.Fatalf("Open failed after recovery: %v", err)
	}
	defer reader.Close()

	if metadata.Version == 0 || metadata.WriterID != "node-a" {
		t.Fatalf("unexpected recovered metadata: %+v", metadata)
	}

	files, err := store.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot failed after recovery: %v", err)
	}
	if len(files) != 1 || files[0].Name != "report.txt" {
		t.Fatalf("unexpected recovered catalogue: %+v", files)
	}
}
