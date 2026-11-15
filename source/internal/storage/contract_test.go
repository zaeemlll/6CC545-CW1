//go:build acceptance

package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
)

func digest(b []byte) string { x := sha256.Sum256(b); return hex.EncodeToString(x[:]) }
func TestPortableNames(t *testing.T) {
	for _, n := range []string{"notes.txt", "A_1-b.csv"} {
		if e := ValidateName(n); e != nil {
			t.Errorf("valid %q: %v", n, e)
		}
	}
	for _, n := range []string{"", "../x", "a/b", "a\\b", ".hidden", "a..b", "CON", "nul.txt", "COM1.csv", "x.", "x:y"} {
		if e := ValidateName(n); e == nil {
			t.Errorf("unsafe %q accepted", n)
		}
	}
}
func TestCommitRoundTripAndIsolation(t *testing.T) {
	s, _ := New(t.TempDir(), 1024)
	body := []byte("hello\n")
	m, e := s.Put(context.Background(), "a.txt", bytes.NewReader(body), int64(len(body)), digest(body))
	if e != nil {
		t.Fatal(e)
	}
	r, got, e := s.Open("a.txt")
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	b, e := io.ReadAll(r)
	if e != nil || !bytes.Equal(b, body) || got != m {
		t.Fatalf("round trip: %q %+v %v", b, got, e)
	}
	list, e := s.Snapshot()
	if e != nil || len(list) != 1 {
		t.Fatalf("snapshot %v %v", list, e)
	}
	list[0].Name = "changed"
	list2, e := s.Snapshot()
	if e != nil || list2[0].Name != "a.txt" {
		t.Fatal("snapshot aliases state")
	}
}
func TestRejectedWritePreservesCommittedFile(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir, 16)
	ctx := context.Background()
	old := []byte("old")
	if _, e := s.Put(ctx, "safe.txt", bytes.NewReader(old), 3, digest(old)); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		b []byte
		n int64
		h string
	}{{[]byte("bad"), 3, digest(old)}, {[]byte("extra"), 2, digest([]byte("ex"))}, {bytes.Repeat([]byte("x"), 17), 17, digest(bytes.Repeat([]byte("x"), 17))}} {
		if _, e := s.Put(ctx, "safe.txt", bytes.NewReader(tc.b), tc.n, tc.h); e == nil {
			t.Fatal("invalid write accepted")
		}
		r, _, e := s.Open("safe.txt")
		if e != nil {
			t.Fatal(e)
		}
		b, _ := io.ReadAll(r)
		r.Close()
		if !bytes.Equal(b, old) {
			t.Fatal("failed write changed data")
		}
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temporary files leaked: %v", entries)
	}
}
func TestCancellationAndAbsentFile(t *testing.T) {
	s, _ := New(t.TempDir(), 1024)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := s.Put(ctx, "x.txt", bytes.NewReader([]byte("x")), 1, digest([]byte("x"))); !errors.Is(e, context.Canceled) {
		t.Fatalf("want cancellation, got %v", e)
	}
	if _, _, e := s.Open("missing.txt"); !errors.Is(e, os.ErrNotExist) {
		t.Fatalf("want not-exist, got %v", e)
	}
}
func TestConcurrentSameNameConsistentSnapshot(t *testing.T) {
	s, _ := New(t.TempDir(), 1024)
	seed := []byte("seed")
	if _, e := s.Put(context.Background(), "same.txt", bytes.NewReader(seed), 4, digest(seed)); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b := []byte(fmt.Sprintf("body-%02d", i))
			if _, e := s.Put(context.Background(), "same.txt", bytes.NewReader(b), int64(len(b)), digest(b)); e != nil {
				t.Error(e)
			}
			r, m, e := s.Open("same.txt")
			if e != nil {
				t.Error(e)
				return
			}
			got, e := io.ReadAll(r)
			r.Close()
			if e != nil || int64(len(got)) != m.Size || digest(got) != m.SHA256 {
				t.Error("inconsistent file snapshot")
			}
		}(i)
	}
	wg.Wait()
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated source read failure")
}

func TestReaderFailurePreservesCommittedFile(t *testing.T) {
	dir := t.TempDir()
	store, err := New(dir, 1024)
	if err != nil {
		t.Fatal(err)
	}

	old := []byte("previous")
	if _, err := store.Put(context.Background(), "safe.txt",
		bytes.NewReader(old), int64(len(old)), digest(old)); err != nil {
		t.Fatal(err)
	}

	replacement := []byte("new-data")
	source := io.MultiReader(bytes.NewReader([]byte("new")), failingReader{})
	_, err = store.Put(context.Background(), "safe.txt",
		source, int64(len(replacement)), digest(replacement))
	if err == nil {
		t.Fatal("expected source reader failure")
	}

	r, _, err := store.Open("safe.txt")
	if err != nil {
		t.Fatal(err)
	}
	got, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, old) {
		t.Fatalf("previous content changed: got %q, want %q", got, old)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "safe.txt" {
		t.Fatalf("unexpected files after failed write: %v", entries)
	}
}

func TestZeroByteFile(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}

	empty := []byte{}
	metadata, err := store.Put(context.Background(), "empty.txt",
		bytes.NewReader(empty), 0, digest(empty))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Size != 0 || metadata.SHA256 != digest(empty) {
		t.Fatalf("unexpected empty-file metadata: %+v", metadata)
	}

	r, gotMetadata, err := store.Open("empty.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	content, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) != 0 || gotMetadata != metadata {
		t.Fatalf("unexpected empty-file result: %q, %+v", content, gotMetadata)
	}
}

func TestSnapshotSortedAndDetached(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"zeta.txt", "alpha.txt", "middle.txt"} {
		body := []byte(name)
		if _, err := store.Put(context.Background(), name,
			bytes.NewReader(body), int64(len(body)), digest(body)); err != nil {
			t.Fatal(err)
		}
	}

	first, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alpha.txt", "middle.txt", "zeta.txt"}
	for i, metadata := range first {
		if metadata.Name != want[i] {
			t.Fatalf("snapshot[%d] = %q, want %q", i, metadata.Name, want[i])
		}
	}

	first[0].Name = "changed.txt"
	second, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if second[0].Name != "alpha.txt" {
		t.Fatal("snapshot modification changed store metadata")
	}
}

func TestConcurrentDifferentNames(t *testing.T) {
	store, err := New(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}

	const count = 12
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("file-%02d.txt", i)
			body := []byte(fmt.Sprintf("content-%02d", i))

			if _, err := store.Put(context.Background(), name,
				bytes.NewReader(body), int64(len(body)), digest(body)); err != nil {
				t.Errorf("Put %s: %v", name, err)
				return
			}

			r, metadata, err := store.Open(name)
			if err != nil {
				t.Errorf("Open %s: %v", name, err)
				return
			}
			got, readErr := io.ReadAll(r)
			_ = r.Close()
			if readErr != nil {
				t.Errorf("Read %s: %v", name, readErr)
				return
			}
			if !bytes.Equal(got, body) ||
				metadata.Size != int64(len(body)) ||
				metadata.SHA256 != digest(body) {
				t.Errorf("inconsistent result for %s", name)
			}
		}(i)
	}
	wg.Wait()

	snapshot, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != count {
		t.Fatalf("snapshot contains %d files, want %d", len(snapshot), count)
	}
}
