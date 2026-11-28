package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func versionTestDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

type versionTestBlockingReader struct {
	body    []byte
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *versionTestBlockingReader) Read(p []byte) (int, error) {
	r.once.Do(func() {
		close(r.started)
		<-r.release
	})
	if len(r.body) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.body)
	r.body = r.body[n:]
	return n, nil
}

func TestPutLocalVersionedIncrementsClock(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	firstBody := []byte("first version")
	first, err := store.PutLocalVersioned(
		context.Background(), "report.txt",
		bytes.NewReader(firstBody), int64(len(firstBody)),
		versionTestDigest(firstBody),
	)
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 1 || first.WriterID != "node-a" {
		t.Fatalf("unexpected first version: %+v", first)
	}

	secondBody := []byte("second version")
	second, err := store.PutLocalVersioned(
		context.Background(), "report.txt",
		bytes.NewReader(secondBody), int64(len(secondBody)),
		versionTestDigest(secondBody),
	)
	if err != nil {
		t.Fatal(err)
	}
	if second.Version != 2 || second.WriterID != "node-a" {
		t.Fatalf("unexpected second version: %+v", second)
	}
}

func TestPutVersionedPreservesRemoteMetadata(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	body := []byte("received from node-b")
	expected := Metadata{
		Name:     "remote.txt",
		Size:     int64(len(body)),
		SHA256:   versionTestDigest(body),
		Version:  7,
		WriterID: "node-b",
	}

	actual, err := store.PutVersioned(
		context.Background(), expected.Name, bytes.NewReader(body), expected,
	)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Version != expected.Version || actual.WriterID != expected.WriterID {
		t.Fatalf("remote metadata was changed: got %+v want %+v", actual, expected)
	}
}

func TestPutRejectsUnversionedWriteWhenEnabled(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	body := []byte("unversioned")
	_, err = store.Put(
		context.Background(), "plain.txt", bytes.NewReader(body),
		int64(len(body)), versionTestDigest(body),
	)
	if !errors.Is(err, ErrVersionedWriteRequired) {
		t.Fatalf("expected ErrVersionedWriteRequired, got %v", err)
	}
}

func TestPutVersionedRejectsLosingVersion(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	currentBody := []byte("current content")
	current := Metadata{
		Name: "report.txt", Size: int64(len(currentBody)),
		SHA256: versionTestDigest(currentBody), Version: 5, WriterID: "node-b",
	}
	if _, err := store.PutVersioned(context.Background(), current.Name, bytes.NewReader(currentBody), current); err != nil {
		t.Fatal(err)
	}

	staleBody := []byte("stale content")
	stale := Metadata{
		Name: current.Name, Size: int64(len(staleBody)),
		SHA256: versionTestDigest(staleBody), Version: 4, WriterID: "node-z",
	}
	if _, err := store.PutVersioned(context.Background(), stale.Name, bytes.NewReader(staleBody), stale); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
}

func TestPutVersionedRejectsStaleWriteAfterNewerCommit(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	initialBody := []byte("initial content")
	initial, err := store.PutLocalVersioned(
		context.Background(), "report.txt",
		bytes.NewReader(initialBody), int64(len(initialBody)),
		versionTestDigest(initialBody),
	)
	if err != nil {
		t.Fatal(err)
	}

	staleBody := []byte("stale remote content")
	stale := Metadata{
		Name: initial.Name, Size: int64(len(staleBody)),
		SHA256:  versionTestDigest(staleBody),
		Version: initial.Version, WriterID: "node-z",
	}
	blockedReader := &versionTestBlockingReader{
		body: staleBody, started: make(chan struct{}), release: make(chan struct{}),
	}
	staleResult := make(chan error, 1)
	go func() {
		_, writeErr := store.PutVersioned(
			context.Background(), stale.Name, blockedReader, stale,
		)
		staleResult <- writeErr
	}()

	<-blockedReader.started

	newerBody := []byte("newer local content")
	newer, newerErr := store.PutLocalVersioned(
		context.Background(), initial.Name,
		bytes.NewReader(newerBody), int64(len(newerBody)),
		versionTestDigest(newerBody),
	)
	close(blockedReader.release)
	if newerErr != nil {
		t.Fatal(newerErr)
	}

	if writeErr := <-staleResult; !errors.Is(writeErr, ErrVersionConflict) {
		t.Fatalf("expected stale write to return ErrVersionConflict, got %v", writeErr)
	}

	reader, actual, err := store.Open(initial.Name)
	if err != nil {
		t.Fatal(err)
	}
	gotBody, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if !bytes.Equal(gotBody, newerBody) {
		t.Fatalf("stored content = %q, want newer content %q", gotBody, newerBody)
	}
	if actual != newer {
		t.Fatalf("stored metadata = %+v, want newer metadata %+v", actual, newer)
	}
}

func TestPutVersionedRejectsEquivocation(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	firstBody := []byte("first claim")
	first := Metadata{
		Name: "report.txt", Size: int64(len(firstBody)),
		SHA256: versionTestDigest(firstBody), Version: 5, WriterID: "node-b",
	}
	if _, err := store.PutVersioned(context.Background(), first.Name, bytes.NewReader(firstBody), first); err != nil {
		t.Fatal(err)
	}

	secondBody := []byte("different claim")
	second := Metadata{
		Name: first.Name, Size: int64(len(secondBody)),
		SHA256: versionTestDigest(secondBody), Version: 5, WriterID: "node-b",
	}
	if _, err := store.PutVersioned(context.Background(), second.Name, bytes.NewReader(secondBody), second); !errors.Is(err, ErrVersionEquivocation) {
		t.Fatalf("expected ErrVersionEquivocation, got %v", err)
	}
}

func TestPutVersionedIdenticalMetadataIsNoOp(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	body := []byte("unchanged content")
	metadata := Metadata{
		Name: "report.txt", Size: int64(len(body)),
		SHA256: versionTestDigest(body), Version: 5, WriterID: "node-b",
	}
	if _, err := store.PutVersioned(context.Background(), metadata.Name, bytes.NewReader(body), metadata); err != nil {
		t.Fatal(err)
	}

	result, err := store.PutVersioned(context.Background(), metadata.Name, bytes.NewReader(body), metadata)
	if err != nil {
		t.Fatal(err)
	}
	if result != metadata {
		t.Fatalf("expected existing metadata %+v, got %+v", metadata, result)
	}
}

func TestPutVersionedUsesWriterIDAsTieBreaker(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	firstBody := []byte("writer a content")
	first := Metadata{
		Name: "report.txt", Size: int64(len(firstBody)),
		SHA256: versionTestDigest(firstBody), Version: 5, WriterID: "node-a",
	}
	if _, err := store.PutVersioned(context.Background(), first.Name, bytes.NewReader(firstBody), first); err != nil {
		t.Fatal(err)
	}

	secondBody := []byte("writer b content")
	second := Metadata{
		Name: first.Name, Size: int64(len(secondBody)),
		SHA256: versionTestDigest(secondBody), Version: 5, WriterID: "node-b",
	}
	if _, err := store.PutVersioned(context.Background(), second.Name, bytes.NewReader(secondBody), second); err != nil {
		t.Fatalf("higher writer ID should win: %v", err)
	}

	files, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != second {
		t.Fatalf("expected winning metadata %+v, got %+v", second, files)
	}
}

func TestVersionedWritesBlockedUntilRecoverySucceeds(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	store.recoveryRequired = true
	store.mu.Unlock()

	body := []byte("recovery protected")
	_, err = store.PutLocalVersioned(
		context.Background(), "report.txt", bytes.NewReader(body),
		int64(len(body)), versionTestDigest(body),
	)
	if err == nil {
		t.Fatal("expected write to be rejected while recovery is required")
	}

	if err := store.Recover(context.Background()); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}

	metadata, err := store.PutLocalVersioned(
		context.Background(), "report.txt", bytes.NewReader(body),
		int64(len(body)), versionTestDigest(body),
	)
	if err != nil {
		t.Fatalf("write should succeed after recovery: %v", err)
	}
	if metadata.Version != 1 || metadata.WriterID != "node-a" {
		t.Fatalf("unexpected metadata after recovery: %+v", metadata)
	}
}

func TestPutVersionedIntegrityFailurePreservesExistingFile(t *testing.T) {
	root := t.TempDir()
	store, err := New(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	originalBody := []byte("committed original content")
	original := Metadata{
		Name:     "report.txt",
		Size:     int64(len(originalBody)),
		SHA256:   versionTestDigest(originalBody),
		Version:  1,
		WriterID: "node-a",
	}
	if _, err := store.PutVersioned(
		context.Background(), original.Name,
		bytes.NewReader(originalBody), original,
	); err != nil {
		t.Fatal(err)
	}

	corruptedBody := []byte("corrupted replacement content")
	replacement := Metadata{
		Name:     original.Name,
		Size:     int64(len(corruptedBody)),
		SHA256:   versionTestDigest([]byte("different content")),
		Version:  2,
		WriterID: "node-b",
	}

	_, err = store.PutVersioned(
		context.Background(), replacement.Name,
		bytes.NewReader(corruptedBody), replacement,
	)
	if !errors.Is(err, ErrIntegrity) {
		t.Fatalf("expected ErrIntegrity, got %v", err)
	}

	gotBody, err := os.ReadFile(filepath.Join(root, original.Name))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBody, originalBody) {
		t.Fatalf("original file changed: got %q, want %q", gotBody, originalBody)
	}

	files, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != original {
		t.Fatalf("original metadata changed: got %+v, want %+v", files, original)
	}

	staged, err := filepath.Glob(filepath.Join(root, versionStagePrefix+"*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(staged) != 0 {
		t.Fatalf("staging files remain after failed write: %v", staged)
	}

	_, err = os.Stat(filepath.Join(root, versionStateDirName, versionJournalName))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected commit journal after failed write: %v", err)
	}
}
