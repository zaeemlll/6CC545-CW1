package syncstate

import (
	"errors"
	"strings"
	"testing"

	"example.edu/6cc545/cw1/internal/storage"
)

func testVersioned(name string, version uint64, writer, hash string) Versioned {
	return Versioned{
		Metadata: storage.Metadata{
			Name:     name,
			Size:     3,
			SHA256:   strings.Repeat(hash, 64),
			Version:  version,
			WriterID: writer,
		},
	}
}

func TestPlanSelectsRemoteWinnersAndSortsByName(t *testing.T) {
	local := []Versioned{
		testVersioned("same.txt", 2, "node-a", "a"),
		testVersioned("older.txt", 3, "node-a", "b"),
	}
	remote := []Versioned{
		testVersioned("z-new.txt", 1, "node-b", "c"),
		testVersioned("older.txt", 2, "node-b", "d"),
		testVersioned("same.txt", 2, "node-a", "a"),
		testVersioned("new.txt", 1, "node-b", "e"),
	}

	got, err := Plan(local, remote)
	if err != nil {
		t.Fatalf("Plan returned error: %v", err)
	}

	wantNames := []string{"new.txt", "z-new.txt"}
	if len(got) != len(wantNames) {
		t.Fatalf("Plan returned %d entries, want %d: %#v", len(got), len(wantNames), got)
	}
	for i, name := range wantNames {
		if got[i].Name != name {
			t.Errorf("entry %d is %q, want %q", i, got[i].Name, name)
		}
	}
}

func TestPlanSelectsHigherVersionAndWriterID(t *testing.T) {
	tests := []struct {
		name   string
		local  Versioned
		remote Versioned
	}{
		{
			name:   "higher version wins",
			local:  testVersioned("file.txt", 1, "node-z", "a"),
			remote: testVersioned("file.txt", 2, "node-a", "b"),
		},
		{
			name:   "higher writer ID wins at equal version",
			local:  testVersioned("file.txt", 2, "node-a", "a"),
			remote: testVersioned("file.txt", 2, "node-b", "b"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Plan([]Versioned{test.local}, []Versioned{test.remote})
			if err != nil {
				t.Fatalf("Plan returned error: %v", err)
			}
			if len(got) != 1 || got[0].WriterID != test.remote.WriterID {
				t.Fatalf("Plan = %#v, want remote winner %#v", got, test.remote)
			}
		})
	}
}

func TestPlanRejectsDuplicateNames(t *testing.T) {
	item := testVersioned("file.txt", 1, "node-a", "a")

	tests := []struct {
		name   string
		local  []Versioned
		remote []Versioned
	}{
		{
			name:  "duplicate local name",
			local: []Versioned{item, item},
		},
		{
			name:   "duplicate remote name",
			remote: []Versioned{item, item},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Plan(test.local, test.remote); err == nil {
				t.Fatal("Plan accepted duplicate file names")
			}
		})
	}
}

func TestPlanRejectsEquivocation(t *testing.T) {
	local := testVersioned("file.txt", 2, "node-a", "a")
	remote := testVersioned("file.txt", 2, "node-a", "b")

	if _, err := Plan([]Versioned{local}, []Versioned{remote}); !errors.Is(err, ErrEquivocation) {
		t.Fatalf("Plan error = %v, want ErrEquivocation", err)
	}
}

func TestPlanRejectsInvalidMetadata(t *testing.T) {
	invalid := testVersioned("file.txt", 0, "node-a", "a")

	if _, err := Plan(nil, []Versioned{invalid}); err == nil {
		t.Fatal("Plan accepted metadata with version zero")
	}
}

func TestPlanEnforcesEntryLimit(t *testing.T) {
	entries := make([]Versioned, maxPlanEntries+1)

	if _, err := Plan(entries, nil); err == nil {
		t.Fatal("Plan accepted too many local entries")
	}
	if _, err := Plan(nil, entries); err == nil {
		t.Fatal("Plan accepted too many remote entries")
	}
}
