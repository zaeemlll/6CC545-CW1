package membership

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPersistentMembershipSurvivesRestart(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), ".cw2-state", "membership.json")
	now := time.Unix(1800000000, 0)

	first, err := NewPersistent(8, 3*time.Second, statePath)
	if err != nil {
		t.Fatal(err)
	}

	if first.Incarnation() != 1 {
		t.Fatalf("first incarnation = %d, want 1", first.Incarnation())
	}

	a := Announcement{
		ID:          "node-b",
		Address:     "https://node-b:9444",
		Incarnation: 2,
		Sequence:    5,
	}
	if err := first.Accept(a, "node-b", now); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("membership state was not written: %v", err)
	}

	restarted, err := NewPersistent(8, 3*time.Second, statePath)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Incarnation() != 2 {
		t.Fatalf("restarted incarnation = %d, want 2", restarted.Incarnation())
	}
	if len(restarted.Snapshot()) != 0 {
		t.Fatal("restart invented active peers")
	}
	if err := restarted.Accept(a, "node-b", now.Add(time.Second)); !errors.Is(err, ErrReplay) {
		t.Fatalf("restart did not reject replay: %v", err)
	}

	a.Sequence++
	if err := restarted.Accept(a, "node-b", now.Add(time.Second)); err != nil {
		t.Fatalf("new sequence was rejected after restart: %v", err)
	}
	if restarted.Incarnation() != 2 {
		t.Fatalf("incarnation after peer update = %d, want 2", restarted.Incarnation())
	}
}

func TestPersistentMembershipDoesNotAcceptWhenSaveFails(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), ".cw2-state", "membership.json")
	table, err := NewPersistent(8, 3*time.Second, statePath)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(statePath)); err != nil {
		t.Fatal(err)
	}

	a := Announcement{
		ID:          "node-b",
		Address:     "https://node-b:9444",
		Incarnation: 1,
		Sequence:    1,
	}
	if err := table.Accept(a, "node-b", time.Now()); err == nil {
		t.Fatal("announcement was accepted despite persistence failure")
	}
	if len(table.Snapshot()) != 0 {
		t.Fatal("failed persistence created an active peer")
	}
	if len(table.Watermarks()) != 0 {
		t.Fatal("failed persistence changed replay watermarks")
	}
}

func TestPersistentMembershipRejectsCorruptState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), ".cw2-state", "membership.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte(`{"version":1,"peers":[`), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := NewPersistent(8, 3*time.Second, statePath); err == nil {
		t.Fatal("corrupt membership state was accepted")
	}
}

func TestPersistentMembershipRejectsSymlinkState(t *testing.T) {
	dir := t.TempDir()
	stateDir := filepath.Join(dir, ".cw2-state")
	if err := os.Mkdir(stateDir, 0700); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, "outside.json")
	if err := os.WriteFile(target, []byte(`{"version":1,"peers":[]}`), 0600); err != nil {
		t.Fatal(err)
	}

	statePath := filepath.Join(stateDir, "membership.json")
	if err := os.Symlink(target, statePath); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}

	if _, err := NewPersistent(8, 3*time.Second, statePath); err == nil {
		t.Fatal("symlink membership state was accepted")
	}
}
