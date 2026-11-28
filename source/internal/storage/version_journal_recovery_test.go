package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func prepareJournalRecovery(t *testing.T, body []byte) (*Store, string, versionJournal) {
	t.Helper()

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

	sum := sha256.Sum256(body)
	metadata := Metadata{
		Name:     "recovered.txt",
		Size:     int64(len(body)),
		SHA256:   hex.EncodeToString(sum[:]),
		Version:  1,
		WriterID: "node-b",
	}
	journal := versionJournal{
		Format:   versionStateFormat,
		TempName: versionStagePrefix + "test-stage",
		Metadata: metadata,
	}
	if err := saveVersionJournal(store.versionStateDir, journal, store.maxBytes); err != nil {
		t.Fatal(err)
	}

	return store, root, journal
}

func TestVersionedRecoveryCompletesStagedCommit(t *testing.T) {
	body := []byte("staged content")
	_, root, journal := prepareJournalRecovery(t, body)

	stagePath := filepath.Join(root, journal.TempName)
	if err := os.WriteFile(stagePath, body, 0600); err != nil {
		t.Fatal(err)
	}

	restarted, err := New(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	files, err := restarted.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != journal.Metadata {
		t.Fatalf("unexpected recovered metadata: %+v", files)
	}
	if _, err := os.Stat(stagePath); !os.IsNotExist(err) {
		t.Fatalf("staged file still exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, journal.Metadata.Name)); err != nil {
		t.Fatalf("recovered destination is missing: %v", err)
	}
}

func TestVersionedRecoveryCompletesRenamedCommit(t *testing.T) {
	body := []byte("renamed content")
	_, root, journal := prepareJournalRecovery(t, body)

	destination := filepath.Join(root, journal.Metadata.Name)
	if err := os.WriteFile(destination, body, 0600); err != nil {
		t.Fatal(err)
	}

	restarted, err := New(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}

	files, err := restarted.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != journal.Metadata {
		t.Fatalf("unexpected recovered metadata: %+v", files)
	}
}

func TestVersionedRecoveryRejectsInvalidJournalFiles(t *testing.T) {
	body := []byte("expected content")
	_, root, journal := prepareJournalRecovery(t, body)

	stagePath := filepath.Join(root, journal.TempName)
	if err := os.WriteFile(stagePath, []byte("corrupted content"), 0600); err != nil {
		t.Fatal(err)
	}

	restarted, err := New(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}
	if err := restarted.Recover(context.Background()); err == nil {
		t.Fatal("recovery accepted a staged file with an incorrect digest")
	}
}

func TestVersionedRecoveryRejectsMissingStageAndDestination(t *testing.T) {
	body := []byte("expected content")
	_, root, _ := prepareJournalRecovery(t, body)

	restarted, err := New(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}

	err = restarted.Recover(context.Background())
	if !errors.Is(err, ErrIntegrity) {
		t.Fatalf("want integrity error for missing stage and destination, got %v", err)
	}
}

func TestVersionedRecoveryRejectsCorruptedDestinationWithoutStage(t *testing.T) {
	body := []byte("expected content")
	_, root, journal := prepareJournalRecovery(t, body)

	destination := filepath.Join(root, journal.Metadata.Name)
	if err := os.WriteFile(destination, []byte("corrupted content"), 0600); err != nil {
		t.Fatal(err)
	}

	restarted, err := New(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if err := restarted.EnableVersioning("node-a"); err != nil {
		t.Fatal(err)
	}

	err = restarted.Recover(context.Background())
	if !errors.Is(err, ErrIntegrity) {
		t.Fatalf("want integrity error for corrupted destination, got %v", err)
	}
}
